import ByzCoinRPC from "../byzcoin-rpc";
import { Darc } from "../../darc/Darc";
import { Argument, ClientTransaction, InstanceID, Instruction } from "../../byzcoin/ClientTransaction";
import { Proof } from "../../byzcoin/Proof";
import { Signer } from "../../darc/Signer";
import { Log } from "../../log";
import { BasicInstance } from "./Instance";

export class DarcInstance extends BasicInstance {
    static readonly contractID = "darc";
    public darc: Darc;

    constructor(public bc: ByzCoinRPC, p: Proof | object = null) {
        super(bc, DarcInstance.contractID, p);
    }

    /**
     * Update the data of this instance
     *
     * @return {Promise<DarcInstance>} - a promise that resolves once the data
     * is up-to-date
     */
    async update(): Promise<DarcInstance> {
        let proof = await this.bc.getProof(new InstanceID(this.darc.getBaseId()));
        this.darc = Darc.decode(proof.value);
        return this;
    }

    static fromObject(bc: ByzCoinRPC, obj: any): DarcInstance {
        return new DarcInstance(bc, obj);
    }

    static async create(bc: ByzCoinRPC, iid: InstanceID, signers: Signer[], d: Darc): Promise<DarcInstance> {
        let inst = Instruction.createSpawn(iid,
            this.contractID,
            [new Argument("darc", Buffer.from(Darc.encode(d).finish()))]);
        let ctx = new ClientTransaction([inst]);
        await ctx.signBy([signers], bc);
        await bc.sendTransactionAndWait(ctx, 5);
        return new DarcInstance(bc, d);
    }

    static async fromProof(bc: ByzCoinRPC, p: Proof): Promise<DarcInstance> {
        await p.matchOrFail(DarcInstance.contractID);
        return new DarcInstance(bc, p);
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * @param bc
     * @param instID
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<DarcInstance> {
        return new DarcInstance(bc, await bc.getProof(iid));
    }

    static darcFromProof(p: Proof): Darc {
        if (p.contractID != DarcInstance.contractID) {
            Log.error("Got non-darc proof: " + p.contractID);
            return null;
        }
        return Darc.decode(p.value);
    }
}
