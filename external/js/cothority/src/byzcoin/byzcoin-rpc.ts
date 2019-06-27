import Long from "long";
import { Rule } from "../darc";
import Darc from "../darc/darc";
import IdentityEd25519 from "../darc/identity-ed25519";
import IdentityWrapper, { IIdentity } from "../darc/identity-wrapper";
import { IConnection, RosterWSConnection, WebSocketConnection } from "../network/connection";
import { Roster } from "../network/proto";
import { SkipBlock } from "../skipchain/skipblock";
import SkipchainRPC from "../skipchain/skipchain-rpc";
import ClientTransaction, { ICounterUpdater } from "./client-transaction";
import ChainConfig from "./config";
import DarcInstance from "./contracts/darc-instance";
import { InstanceID } from "./instance";
import Proof from "./proof";
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

export const currentVersion = 1;

const CONFIG_INSTANCE_ID = Buffer.alloc(32, 0);

export default class ByzCoinRPC implements ICounterUpdater {

    get genesisID(): InstanceID {
        return this.genesis.computeHash();
    }

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
     * @param roster        The roster to ask for the config and darc
     * @param skipchainID   The genesis block identifier
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @returns a promise that resolves with the initialized ByzCoin instance
     */
    static async fromByzcoin(roster: Roster, skipchainID: Buffer, waitMatch: number = 0, interval: number = 1000):
        Promise<ByzCoinRPC> {
        const rpc = new ByzCoinRPC();
        rpc.conn = new RosterWSConnection(roster, "ByzCoin");

        const skipchain = new SkipchainRPC(roster);
        rpc.genesis = await skipchain.getSkipBlock(skipchainID);
        rpc.latest = rpc.genesis;

        const ccProof = await rpc.getProof(CONFIG_INSTANCE_ID, waitMatch, interval);
        rpc.config = ChainConfig.fromProof(ccProof);

        const di = await DarcInstance.fromByzcoin(rpc, ccProof.stateChangeBody.darcID, waitMatch, interval);
        rpc.genesisDarc = di.darc;

        return rpc;
    }

    /**
     * Create a new byzcoin chain and return a associated RPC
     * @param roster        The roster to use to create the genesis block
     * @param darc          The genesis darc
     * @param blockInterval The interval of block creation in nanoseconds
     */
    static async newByzCoinRPC(roster: Roster, darc: Darc, blockInterval: Long): Promise<ByzCoinRPC> {
        const rpc = new ByzCoinRPC();
        rpc.conn = new WebSocketConnection(roster.list[0].getWebSocketAddress(), "ByzCoin");
        rpc.genesisDarc = darc;
        rpc.config = new ChainConfig({blockInterval});

        const req = new CreateGenesisBlock({
            blockInterval,
            darcContractIDs: [DarcInstance.contractID],
            genesisDarc: darc,
            roster,
            version: currentVersion,
        });

        const ret = await rpc.conn.send<CreateGenesisBlockResponse>(req, CreateGenesisBlockResponse);
        rpc.genesis = ret.skipblock;
        rpc.latest = ret.skipblock;
        await rpc.updateConfig();

        return rpc;
    }

    private genesisDarc: Darc;
    private config: ChainConfig;
    private genesis: SkipBlock;
    private latest: SkipBlock;
    private conn: IConnection;

    protected constructor() {}

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
    sendTransactionAndWait(transaction: ClientTransaction, wait: number = 10): Promise<AddTxResponse> {
        const req = new AddTxRequest({
            inclusionwait: wait,
            skipchainID: this.genesis.hash,
            transaction,
            version: currentVersion,
        });

        return this.conn.send(req, AddTxResponse);
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
        if (!this.latest) {
            throw new Error("no latest block found");
        }

        return this.getProofFrom(this.latest, id, waitMatch, interval);
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state. The proof starts from the block given in parameter.
     * Caution: If you need to pass the Proof onwards to another server,
     * you must use getProof in order to create a complete standalone
     * proof starting from the genesis block.
     *
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

        this.latest = reply.proof.latest;

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
        return rep.counters.map((c) => c.add(add));
    }

    /**
     * checks the authorization of a set of identities with respect to a given darc. This calls
     * an OmniLedger node and trusts it to return the name of the actions that a hypotethic set of
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
}
