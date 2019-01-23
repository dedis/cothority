import {KeyPair} from "~/lib/KeyPair";
import {Darc, Rule, Rules} from "../darc/Darc";
import {Log} from "~/lib/Log";
import {RosterSocket, Socket} from "~/lib/network/NSNet";
import {RequestPath} from "~/lib/network/RequestPath";
import {objToProto, Root} from "~/lib/cothority/protobuf/Root";
import {IdentityEd25519} from "~/lib/cothority/darc/IdentityEd25519";
import {Proof} from "~/lib/cothority/byzcoin/Proof";
import {Signer} from "~/lib/cothority/darc/Signer";
import {SignerEd25519} from "~/lib/cothority/darc/SignerEd25519";
import * as Long from "long";
import {Roster} from "~/lib/network/Roster";
import {ClientTransaction, InstanceID} from "~/lib/cothority/byzcoin/ClientTransaction";
import {DarcInstance} from "~/lib/cothority/byzcoin/contracts/DarcInstance";
import {Identity} from "~/lib/cothority/darc/Identity";

const UUID = require("pure-uuid");
const crypto = require("crypto-browserify");

export const currentVersion = 1;

export class ByzCoinRPC {
    admin: Signer;

    constructor(public socket: Socket,
                public bcID: Buffer,
                public genesisDarc: Darc,
                public config: ChainConfig) {
    }

    /**
     * Sends a transaction to byzcoin and waits for up to 'wait' blocks for the
     * transaction to be included in the global state. If more than 'wait' blocks
     * are created and the transaction is not included, an exception will be raised.
     *
     * @param {ClientTransaction} transaction - is the client transaction holding
     * one or more instructions to be sent to byzcoin.
     * @param {number} wait - indicates the number of blocks to wait for the
     * transaction to be included
     * @return {Promise} - a promise that gets resolved if the block has been included
     */
    async sendTransactionAndWait(transaction: ClientTransaction, wait: number = 5): Promise<any> {
        let addTxRequest = {
            version: currentVersion,
            skipchainid: this.bcID,
            transaction: transaction.toObject(),
            inclusionwait: wait
        };
        await this.socket.send("AddTxRequest", "AddTxResponse", addTxRequest);
        return null;
    }

    async updateConfig(): Promise<any> {
        let configIID = new InstanceID(new Buffer(32));
        let pr = await this.getProof(configIID);
        ByzCoinRPC.checkProof(pr, configIID, "config");
        this.config = ChainConfig.fromProof(pr);

        let darcIID = new InstanceID(pr.stateChangeBody.darcID);
        let genesisDarcProof = await this.getProof(darcIID);
        ByzCoinRPC.checkProof(genesisDarcProof, darcIID, "darc");
        this.genesisDarc = DarcInstance.darcFromProof(genesisDarcProof);
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state.

     * @param {Buffer} id - the instance key
     * @return {Promise<Proof>}
     */
    async getProof(id: InstanceID): Promise<Proof> {
        return ByzCoinRPC.getProof(this.socket, this.bcID, id);
    }

    async getSignerCounters(ids: Identity[], add: number = 0): Promise<Long[]> {
        let req = {
            skipchainid: this.bcID,
            signerids: ids.map(id => id.toString()),
        };
        let reply = await this.socket.send("byzcoin.GetSignerCounters",
            "byzcoin.GetSignerCountersResponse",
            req);
        return reply.counters.map((c: Long) => c.add(1));
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state.
     *
     * @param {Socket|LeaderSocket|RosterSocket} socket - the socket to communicate with the conode
     * @param {Buffer} skipchainId - the skipchain ID (the ID of it's genesis block)
     * @param {Buffer} key - the instance key
     * @return {Promise<Proof>}
     */
    static async getProof(socket: Socket, skipchainId: Buffer,
                          key: InstanceID): Promise<Proof> {
        const getProofMessage = {
            version: currentVersion,
            id: skipchainId,
            key: key.iid
        };
        let reply = await socket.send("GetProof", "GetProofResponse", getProofMessage);
        return new Proof(reply.proof, key);
    }

    /**
     * Check the validity of the proof
     *
     * @param {Proof} proof
     * @param {string} expectedContract
     * @throws {Error} if the proof is not valid
     */
    static checkProof(proof: Proof, iid: InstanceID, expectedContract: string) {
        if (!proof.inclusionproof.matches(iid)) {
            throw "it is a proof of absence";
        }
        let contract = proof.stateChangeBody.contractID;
        if (!(contract === expectedContract)) {
            throw "contract name is not " + expectedContract + ", got " + contract;
        }
    }

    static defaultGenesisMessage(roster: any, rules: string[], ids: any[]): GenesisMessage {
        if (ids.length == 0) {
            throw new Error("no identities");
        }

        let d = Darc.fromRulesDesc(Rules.fromOwnersSigners(ids, ids), "genesis darc");
        rules.forEach(r => {
            d.rules.list.push(Rule.fromIdentities(r, ids, "|"));
        });

        let rosterPubs = roster.list.map(l => {
            return "ed25519:" + l.public.point;
        });
        d.rules.list.push(Rule.fromIdentities("invoke:view_change", rosterPubs, "|"));

        return new GenesisMessage(1, roster.toObject(), d, 1e9);
    }

    static async newLedger(roster: any, rules: string[] = []): Promise<ByzCoinRPC> {
        let admin = new KeyPair();
        let ids = [new IdentityEd25519(admin._public.point)];
        let msg = this.defaultGenesisMessage(roster, rules, ids);
        let socket = new RosterSocket(roster, RequestPath.BYZCOIN);
        let reply = await socket.send(RequestPath.BYZCOIN_CREATE_GENESIS, RequestPath.BYZCOIN_CREATE_GENESIS_RESPONSE, msg);
        let adminSigner = new SignerEd25519(admin._public.point, admin._private.scalar);
        let bc = new ByzCoinRPC(socket, Buffer.from(reply.skipblock.hash), msg.genesisdarc, null);
        bc.admin = adminSigner;
        await bc.updateConfig();
        return bc;
    }

    static async fromByzcoin(s: Socket, bcID: Buffer): Promise<ByzCoinRPC> {
        let ccProof = await ByzCoinRPC.getProof(s, bcID, new InstanceID(new Buffer(32)));
        let cc = ChainConfig.fromProof(ccProof);
        let gdProof = await ByzCoinRPC.getProof(s, bcID, new InstanceID(ccProof.stateChangeBody.darcID));
        let gd = DarcInstance.darcFromProof(gdProof);
        return new ByzCoinRPC(s, bcID, gd, cc);
    }
}

export class ChainConfig {
    roster: Roster;
    blockinterval: Long;
    maxblocksize: Long;

    constructor(cc: any) {
        this.roster = Roster.fromObject(cc.roster);
        this.blockinterval = cc.blockinterval;
        this.maxblocksize = cc.maxblocksize;
    }

    toProto(): Buffer {
        return objToProto(this, "ChainConfig");
    }

    static fromProto(buf: Buffer): ChainConfig {
        const requestModel = Root.lookup("ChainConfig");
        return new ChainConfig(requestModel.decode(buf));
    }

    static fromProof(pr: Proof): ChainConfig {
        return this.fromProto(pr.stateChangeBody.value);
    }
}

export class GenesisMessage {
    constructor(public version: number,
                public roster: Roster,
                public genesisdarc: Darc,
                public blockinterval: number,
    ) {
    }
}