import Long from "long";
import { BehaviorSubject } from "rxjs";
import { tap } from "rxjs/internal/operators/tap";
import { distinctUntilChanged, filter, map, mergeMap } from "rxjs/operators";
import { Rule } from "../darc";
import Darc from "../darc/darc";
import IdentityEd25519 from "../darc/identity-ed25519";
import IdentityWrapper, { IIdentity } from "../darc/identity-wrapper";
import { WebSocketAdapter } from "../network";
import {
    IConnection,
    LeaderConnection,
    RosterWSConnection,
} from "../network/connection";
import { Roster } from "../network/proto";
import { SkipBlock } from "../skipchain/skipblock";
import SkipchainRPC from "../skipchain/skipchain-rpc";
import ClientTransaction, { ICounterUpdater } from "./client-transaction";
import ChainConfig from "./config";
import DarcInstance from "./contracts/darc-instance";
import { InstanceID } from "./instance";
import Proof from "./proof";
import { DataHeader } from "./proto";
import CheckAuthorization, { CheckAuthorizationResponse } from "./proto/check-auth";
import {
    AddTxRequest,
    AddTxResponse,
    CreateGenesisBlock,
    CreateGenesisBlockResponse,
    GetProof,
    GetProofResponse,
    GetSignerCounters,
    GetSignerCountersResponse,
} from "./proto/requests";
import { StreamingRequest, StreamingResponse } from "./proto/stream";

export const currentVersion = 2;

export const CONFIG_INSTANCE_ID = Buffer.alloc(32, 0);

/**
 * ByzCoinRPC represents one byzcoin-representation.
 *
 * TODO:
 * Split this into two classes:
 * - one with only static methods mapped to the API of byzcoin
 * - one representing a single byzcoin-instance
 */
export default class ByzCoinRPC implements ICounterUpdater {

    get genesisID(): InstanceID {
        return this.genesis.computeHash();
    }

    get latest(): SkipBlock {
        return new SkipBlock(this._latest);
    }

    static readonly serviceName = "ByzCoin";

    /**
     * Helper to create a genesis darc
     * @param signers       Authorized signers
     * @param roster        Roster that will be used
     * @param description   An optional description for the chain
     */
    static makeGenesisDarc(signers: IIdentity[], roster: Roster, description?: string): Darc {
        if (signers.length === 0) {
            throw new Error("no identities");
        }

        const d = Darc.createBasic(signers, signers, Buffer.from(description || "Genesis darc"));
        roster.list.forEach((srvid) => {
            d.addIdentity("invoke:config.view_change", IdentityEd25519.fromPoint(srvid.getPublic()), Rule.OR);
        });

        signers.forEach((signer) => {
            d.addIdentity("spawn:darc", signer, Rule.OR);
            d.addIdentity("invoke:config.update_config", signer, Rule.OR);
        });

        return d;
    }

    /**
     * Recreate a byzcoin RPC from a given roster
     * @param nodes either a roster or an initialized connection
     * @param skipchainID   The genesis block identifier
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @param latest if given, use this to prove the current state of the blockchain. Needs to be trusted!
     * @param storage to be used to store instance caches
     * @returns a promise that resolves with the initialized ByzCoin instance
     */
    static async fromByzcoin(nodes: Roster | IConnection, skipchainID: Buffer, waitMatch: number = 0,
                             interval: number = 1000, latest?: SkipBlock,
                             storage?: IStorage):
        Promise<ByzCoinRPC> {
        const rpc = new ByzCoinRPC();
        if (nodes instanceof Roster) {
            rpc.conn = new RosterWSConnection(nodes, ByzCoinRPC.serviceName);
        } else {
            rpc.conn = nodes.copy(ByzCoinRPC.serviceName);
        }

        const skipchain = new SkipchainRPC(rpc.conn);
        rpc.genesis = await skipchain.getSkipBlock(skipchainID);
        rpc._latest = latest !== undefined ? latest : rpc.genesis;

        const ccProof = await rpc.getProofFromLatest(CONFIG_INSTANCE_ID, waitMatch, interval);
        rpc.config = ChainConfig.fromProof(ccProof);
        const di = await DarcInstance.fromByzcoin(rpc, ccProof.stateChangeBody.darcID, waitMatch, interval);

        rpc.genesisDarc = di.darc;
        rpc.db = storage;

        return rpc;
    }

    /**
     * Create a new byzcoin chain and returns an associated RPC
     * @param roster        The roster to use to create the genesis block
     * @param darc          The genesis darc
     * @param blockInterval The interval of block creation in nanoseconds
     * @param storage       To be used to store instance caches
     */
    static async newByzCoinRPC(roster: Roster, darc: Darc, blockInterval: Long,
                               storage?: IStorage): Promise<ByzCoinRPC> {
        const leader = new LeaderConnection(roster, ByzCoinRPC.serviceName);
        const req = new CreateGenesisBlock({
            blockInterval,
            darcContractIDs: [DarcInstance.contractID],
            genesisDarc: darc,
            roster,
            version: currentVersion,
        });

        const ret = await leader.send<CreateGenesisBlockResponse>(req, CreateGenesisBlockResponse);
        return ByzCoinRPC.fromByzcoin(roster, ret.skipblock.hash,
            undefined, undefined, undefined, storage);
    }
    private static staticCounters = new Map<string, Map<string, Long>>();
    private newBlockWS: WebSocketAdapter;
    private genesisDarc: Darc;
    private newBlock: BehaviorSubject<SkipBlock>;
    private config: ChainConfig;
    private genesis: SkipBlock;
    private conn: IConnection;
    private db: IStorage;
    private cache = new Map<InstanceID, BehaviorSubject<Proof>>();

    private _latest: SkipBlock;

    protected constructor() {
    }

    /**
     * Getter for the genesis darc
     * @returns the genesis darc
     */
    getDarc(): Darc {
        return this.genesisDarc;
    }

    /**
     * Getter for the chain configuration
     * @returns the configuration
     */
    getConfig(): ChainConfig {
        return this.config;
    }

    /**
     * Getter for the genesis block
     * @returns the genesis block
     */
    getGenesis(): SkipBlock {
        return this.genesis;
    }

    /**
     * Getter for the ByzCoin protocol version of the
     * latest block known by the RPC client.
     */
    getProtocolVersion(): number {
        const header = DataHeader.decode(this._latest.data);

        return header.version;
    }

    /**
     * Returns an observable proof for an instance that will be
     * automatically updated whenever the instance is changed.
     * It returns a BehaviorSubject, which is a special observable that
     * keeps the current value.
     * It also caches all requests, so if there are two requests for the
     * same id, no new observable will be created.
     * It connects to `getNewBlocks` to be informed whenever a new block is
     * created.
     * @param id of the instance to return
     * @throws an error if the instance does not exist
     */
    async instanceObservable(id: InstanceID): Promise<BehaviorSubject<Proof>> {
        const bs = this.cache.get(id);
        if (bs !== undefined) {
            return bs;
        }

        // Check if the db already has a version, which might be outdated,
        // but still better than to wait for the network.
        // might be old, but be informed as soon as the correct values arrive.
        // This makes it possible to have a quick display of values that
        const idStr = id.toString("hex");
        const proofBuf = await this.db.get(idStr);
        let dbProof: Proof;
        if (proofBuf === undefined) {
            dbProof = await this.getProofFromLatest(id);
        } else {
            dbProof = Proof.decode(proofBuf);
        }
        if (!dbProof.exists(id)) {
            throw new Error("this instance does not exist");
        }

        // Create a new BehaviorSubject with the proof, which might not be
        // current, but a best guess from the db of a previous session.
        const bsNew = new BehaviorSubject(dbProof);
        this.cache.set(id, bsNew);

        // Set up a pipe from the block to fetch new versions if a new block
        // arrives.
        // Start with an observable that emits each new block as it arrives.
        (await this.getNewBlocks())
            .pipe(
                // Make sure only newer blocks than the proof are taken into
                // account
                filter((block) => block.index > dbProof.latest.index),
                // Get a new proof of the instance
                mergeMap(() => this.getProofFromLatest(id)),
                // Don't emit proofs that are already known
                distinctUntilChanged((a, b) =>
                    a.stateChangeBody.version.equals(b.stateChangeBody.version)),
                // Store new proofs in the db for later use
                tap((proof) =>
                    this.db.set(idStr, Buffer.from(Proof.encode(proof).finish()))),
                // Link to the BehaviorSubject
            ).subscribe(bsNew);

        // Return the BehaviorSubject - the pipe will continue to run in the
        // background and check if the proof changed on the emission of
        // every new block.
        return bsNew;
    }

    /**
     * Defines how many nodes will be contacted in parallel when sending a message
     * @param p nodes to contact in parallel
     */
    setParallel(p: number) {
        this.conn.setParallel(p);
    }

    /**
     * Sends a transaction to byzcoin and waits for up to 'wait' blocks for the
     * transaction to be included in the global state. If more than 'wait' blocks
     * are created and the transaction is not included, an exception will be raised.
     *
     * @param transaction is the client transaction holding
     * one or more instructions to be sent to byzcoin.
     * @param wait indicates the number of blocks to wait for the
     * transaction to be included
     * @returns a promise that gets resolved if the block has been included
     */
    async sendTransactionAndWait(transaction: ClientTransaction, wait: number = 10): Promise<AddTxResponse> {
        const req = new AddTxRequest({
            inclusionwait: wait,
            skipchainID: this.genesis.hash,
            transaction,
            version: currentVersion,
        });
        const counters = this.counters();

        // The error might be in the response, so we look for it and then reject the promise.
        const resp = await this.conn.send(req, AddTxResponse) as AddTxResponse;
        // If the transaction has been successful, we update all cached counters. If the transaction
        // failed, all involved counters are reset and will have to be fetched again.
        transaction.instructions.forEach((instruction) => {
            instruction.signerIdentities.forEach((signer, i) => {
                if (resp.error.length === 0) {
                    counters.set(signer.toString(), instruction.signerCounter[i]);
                } else {
                    counters.delete(signer.toString());
                }
            });
        });
        if (resp.error.length === 0) {
            return Promise.resolve(resp);
        }
        return Promise.reject(new Error(resp.error));
    }

    /**
     * Get the latest configuration for the chain and update the local
     * cache
     */
    async updateConfig(): Promise<void> {
        const pr = await this.getProofFromLatest(CONFIG_INSTANCE_ID);
        this.config = ChainConfig.fromProof(pr);

        const darcIID = pr.stateChangeBody.darcID;
        const genesisDarcInstance = await DarcInstance.fromByzcoin(this, darcIID);

        this.genesisDarc = genesisDarcInstance.darc;
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state. The proof always starts from the genesis block.
     *
     * @param id the instance key
     * @param waitMatch number of milliseconds to wait if the proof is false
     * @param interval how long to wait before checking for a match again
     * @return a promise that resolves with the proof, rejecting otherwise
     */
    async getProof(id: Buffer, waitMatch: number = 0, interval: number = 1000): Promise<Proof> {
        if (!this.genesis) {
            throw new Error("RPC not initialized with the genesis block");
        }

        return this.getProofFrom(this.genesis, id, waitMatch, interval);
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state. The proof starts from the latest known block.
     * Caution: If you need to pass the Proof onwards to another server,
     * you must use getProof in order to create a complete standalone
     * proof starting from the genesis block.
     *
     * @param id the instance key
     * @param waitMatch number of milliseconds to wait if the proof is false
     * @param interval how long to wait before checking for a match again
     * @return a promise that resolves with the proof, rejecting otherwise
     */
    async getProofFromLatest(id: Buffer, waitMatch: number = 0, interval: number = 1000): Promise<Proof> {
        if (this._latest === undefined) {
            throw new Error("no latest block found");
        }

        return this.getProofFrom(this._latest, id, waitMatch, interval);
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state. The proof starts from the block given in parameter.
     * Caution: If you need to pass the Proof onwards to another server,
     * you must use getProof in order to create a complete standalone
     * proof starting from the genesis block.
     *
     * @param from skipblock to start
     * @param id the instance key
     * @param waitMatch number of milliseconds to wait if the proof is false
     * @param interval how long to wait before checking for a match again
     * @return a promise that resolves with the proof, rejecting otherwise
     */
    async getProofFrom(from: SkipBlock, id: Buffer, waitMatch: number = 0, interval: number = 1000): Promise<Proof> {
        const req = new GetProof({
            id: from.hash,
            key: id,
            version: currentVersion,
        });

        const reply = await this.conn.send<GetProofResponse>(req, GetProofResponse);
        if (waitMatch > 0 && !reply.proof.exists(id)) {
            return new Promise((resolve, reject) => {
                setTimeout(() => {
                    this.getProofFrom(from, id, waitMatch - interval, interval).then(resolve, reject);
                }, interval);
            });
        }

        const err = reply.proof.verifyFrom(from);
        if (err) {
            throw err;
        }

        this._latest = reply.proof.latest;

        return reply.proof;
    }

    /**
     * Get the latest counter for the given signers and increase it with a given value
     *
     * @param ids The identifiers of the signers
     * @param add The increment
     * @returns the ordered list of counters
     */
    async getSignerCounters(ids: IIdentity[], add: number = 0): Promise<Long[]> {
        const req = new GetSignerCounters({
            signerIDs: ids.map((id) => id.toString()),
            skipchainID: this.genesis.hash,
        });

        const rep = await this.conn.send<GetSignerCountersResponse>(req, GetSignerCountersResponse);
        const counters = this.counters();
        ids.forEach((counter, i) => counters.set(counter.toString(), rep.counters[i]));
        return rep.counters.map((c) => c.add(add));
    }

    /**
     * This keeps track of the know counters, thus speeding up signing with known keys. It
     * also solves the problem of counters sometimes being out of synch with the chain.
     * After the call returns, all signers are in ClientTransaction.counters.
     *
     * @param rpc should allow updates for counters
     * @param signers the signers needed
     */
    async updateCachedCounters(signers: IIdentity[]): Promise<Long[]> {
        const counters = this.counters();
        const newSigners = signers.filter((signer) => {
            return counters.has(signer.toString()) === false;
        });
        if (newSigners.length > 0) {
            await this.getSignerCounters(newSigners, 0);
        }
        return signers.map((signer) => {
            return counters.get(signer.toString());
        });
    }

    /**
     * Clears the counters, in case of an error.
     */
    clearCounters() {
        this.counters().clear();
    }

    /**
     * Returns the next value for the counter and updates the cache.
     */
    getNextCounter(signer: IIdentity): Long {
        const counters = this.counters();
        const c = counters.get(signer.toString()).add(1);
        counters.set(signer.toString(), c);
        return c;
    }

    /**
     * checks the authorization of a set of identities with respect to a given darc. This calls
     * an OmniLedger node and trusts it to return the name of the actions that a hypothetical set of
     * signatures from the given identities can execute using the given darc.
     *
     * This is useful if a darc delegates one or more actions to other darc, who delegate also, so
     * this call will test what actions are possible to be executed.
     *
     * @param darcID the base darc whose actions are verified
     * @param identities the set of identities that are hypothetically signing
     */
    async checkAuthorization(byzCoinID: InstanceID, darcID: InstanceID, ...identities: IdentityWrapper[])
        : Promise<string[]> {
        const req = new CheckAuthorization({
            byzcoinID: byzCoinID,
            darcID,
            identities,
            version: currentVersion,
        });

        const reply = await this.conn.send<CheckAuthorizationResponse>(req, CheckAuthorizationResponse);
        return reply.actions;
    }

    /**
     * The returned BehaviorSubject replays all new blocks to all listeners.
     * The streaming is attached to the first node of the connectionlist.
     */
    async getNewBlocks(): Promise<BehaviorSubject<SkipBlock>> {
        if (this.newBlock !== undefined) {
            return this.newBlock;
        }
        if (this._latest === undefined) {
            await this.getProofFromLatest(CONFIG_INSTANCE_ID);
        }
        this.newBlock = new BehaviorSubject(this._latest);
        const msgBlock = new StreamingRequest({
            id: this.genesisID,
        });
        this.conn.sendStream<StreamingResponse>(msgBlock,
            StreamingResponse).pipe(map(([sr, ws]) => {
            this.newBlockWS = ws;
            return sr.block;
        })).subscribe(this.newBlock);
        return this.newBlock;
    }

    /**
     * Closes an eventual newBlock websocket. All connected BehaviorSubjects
     * will get a 'completed' message.
     */
    closeNewBlocks() {
        if (this.newBlock) {
            this.newBlockWS.close(1000);
            this.newBlock = undefined;
        }
    }

    private counters(): Map<string, Long> {
        const idStr = this.genesisID.toString("hex");
        if (!ByzCoinRPC.staticCounters.has(idStr)) {
            ByzCoinRPC.staticCounters.set(idStr, new Map<string, Long>());
        }
        return ByzCoinRPC.staticCounters.get(idStr);
    }

}

/**
 * IStorage represents a storage backend - either a local cache, or a db
 * that stays around between sessions.
 */
export interface IStorage {
    get(key: string): Promise<Buffer | undefined>;

    set(key: string, value: Buffer): Promise<void>;
}

/**
 * LocalCache wraps a Map<string, Buffer> to be used by the Marshaller.
 */
export class LocalCache implements IStorage {
    private cache = new Map<string, Buffer>();

    async get(key: string): Promise<Buffer | undefined> {
        return this.cache.get(key);
    }

    async set(key: string, value: Buffer): Promise<void> {
        this.cache.set(key, value);
    }
}
