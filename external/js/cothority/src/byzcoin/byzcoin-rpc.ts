import Darc from "../darc/darc";
import Proof from "./proof";
import Signer from "../darc/signer";
import { Roster } from "../network/proto";
import ClientTransaction from "./client-transaction";
import Identity from "../darc/identity";
import { Connection, RosterWSConnection, WebSocketConnection } from "../network/connection";
import { AddTxRequest, AddTxResponse, CreateGenesisBlock, CreateGenesisBlockResponse, GetProof, GetProofResponse, GetSignerCounters, GetSignerCountersResponse } from "./proto";
import { SkipBlock } from "../skipchain/skipblock";
import ChainConfig from "./config";
import IdentityEd25519 from "../darc/identity-ed25519";
import Rules from "../darc/rules";

export const currentVersion = 1;

export default class ByzCoinRPC {
    private admin: Signer;
    private conn: Connection;
    private genesisDarc: Darc;
    private config: ChainConfig;
    private genesis: SkipBlock;

    protected constructor() { }

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
    sendTransactionAndWait(transaction: ClientTransaction, wait: number = 5): Promise<AddTxResponse> {
        const req = new AddTxRequest({
            version: currentVersion,
            skipchainID: this.genesis.hash,
            transaction,
            inclusionwait: wait,
        });
        
        return this.conn.send(req, AddTxResponse);
    }

    async updateConfig(): Promise<void> {
        let configIID = Buffer.alloc(32, 0);
        let pr = await this.getProof(configIID);
        ByzCoinRPC.checkProof(pr, configIID, "config");
        this.config = ChainConfig.fromProof(pr);

        let darcIID = pr.stateChangeBody.darcID;
        let genesisDarcProof = await this.getProof(darcIID);
        ByzCoinRPC.checkProof(genesisDarcProof, darcIID, "darc");
        this.genesisDarc = Darc.fromProof(genesisDarcProof);
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state.

     * @param {Buffer} id - the instance key
     * @return {Promise<Proof>}
     */
    async getProof(id: Buffer): Promise<Proof> {
        const req = new GetProof({
            version: currentVersion,
            id: this.genesis.hash,
            key: id,
        });

        const reply = await this.conn.send<GetProofResponse>(req, GetProofResponse);
        return reply.proof;
    }

    async getSignerCounters(ids: Identity[], add: number = 0): Promise<Long[]> {
        const req = new GetSignerCounters({
            skipchainid: this.genesis.hash,
            signerids: ids.map(id => id.toString()),
        });
        
        const rep = await this.conn.send<GetSignerCountersResponse>(req, GetSignerCountersResponse);

        return rep.counters.map(c => c.add(add));
    }

    /**
     * Check the validity of the proof
     *
     * @param {Proof} proof
     * @param {string} expectedContract
     * @throws {Error} if the proof is not valid
     */
    static checkProof(proof: Proof, iid: Buffer, expectedContract: string) {
        if (!proof.inclusionproof.matches(iid)) {
            throw "it is a proof of absence";
        }
        let contract = proof.stateChangeBody.contractID;
        if (!proof.matchContract(expectedContract)) {
            throw "contract name is not " + expectedContract + ", got " + contract;
        }
    }

    static makeGenesisDarc(signers: Identity[], roster: Roster, description?: string): Darc {
        if (signers.length == 0) {
            throw new Error("no identities");
        }

        const d = Darc.newDarc(signers, signers, Buffer.from(description || "Genesis darc"));
        roster.list.forEach((srvid) => {
            d.addIdentity('view_change', new IdentityEd25519({ point: srvid.public }), Rules.OR);
        });

        signers.forEach((signer) => {
            d.addIdentity('spawn:darc', signer, Rules.OR);
            d.addIdentity('invoke:update_config', signer, Rules.OR);
        });

        return d;
    }

    static async fromByzcoin(roster: Roster, id: Buffer): Promise<ByzCoinRPC> {
        const rpc = new ByzCoinRPC();
        rpc.conn = new RosterWSConnection(roster, 'ByzCoin');

        const ccProof = await rpc.getProof(Buffer.alloc(32, 0));
        rpc.config = ChainConfig.fromProof(ccProof);

        const gdProof = await rpc.getProof(ccProof.stateChangeBody.darcID);
        rpc.genesisDarc = Darc.fromProof(gdProof);

        return new ByzCoinRPC();
    }

    static async newByzCoinRPC(roster: Roster, darc: Darc, blockInterval: number): Promise<ByzCoinRPC> {
        const rpc = new ByzCoinRPC();
        rpc.conn = new WebSocketConnection(roster.list[0].getWebSocketAddress(), 'ByzCoin');
        rpc.genesisDarc = darc;
        rpc.config = new ChainConfig({ blockInterval });

        const req = new CreateGenesisBlock({
            version: currentVersion,
            roster,
            genesisDarc: darc,
            blockInterval,
        });

        const ret = await rpc.conn.send<CreateGenesisBlockResponse>(req, CreateGenesisBlockResponse);

        rpc.genesis = ret.skipblock;

        return rpc;
    }
}
