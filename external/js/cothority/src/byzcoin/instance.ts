import { Properties } from "protobufjs/light";
import Log from "../log";
import ByzCoinRPC from "./byzcoin-rpc";
import Proof from "./proof";

export type InstanceID = Buffer;

/**
 * Instance with basic information
 */
export default class Instance {
    /**
     * Create an instance from a proof
     * @param id the instance-id to extract
     * @param p The proof
     * @returns the instance
     */
    static fromProof(id: InstanceID, p: Proof): Instance {
        if (!p.exists(id)) {
            throw new Error(`key not in proof: ${id.toString("hex")}`);
        }

        return new Instance({id, contractID: p.contractID, darcID: p.darcID, data: p.value});
    }

    /**
     * Create an instance after requesting its proof to byzcoin
     * @param rpc   The RPC to use
     * @param iid    The ID of the instance
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @returns the instance if it exists
     */
    static async fromByzcoin(rpc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<Instance> {
        const p = await rpc.getProofFromLatest(iid, waitMatch, interval);

        return Instance.fromProof(iid, p);
    }

    /**
     * Returns an instance from a previously toBytes() call.
     * @param buf
     */
    static fromBytes(buf: Buffer): Instance {
        const obj = JSON.parse(buf.toString());
        return new Instance({
            contractID: obj.contractID,
            darcID: Buffer.from(obj.darcID),
            data: Buffer.from(obj.data),
            id: Buffer.from(obj.id),
        });
    }

    readonly id: InstanceID;
    readonly contractID: string;
    darcID: InstanceID;
    data: Buffer;

    constructor(init: Properties<Instance> | Instance) {
        this.id = init.id;
        this.contractID = init.contractID;
        this.darcID = init.darcID;
        this.data = init.data;
    }

    /**
     * Returns a byte representation of the Instance.
     */
    toBytes(): Buffer {
        return Buffer.from(JSON.stringify(new Instance(this)));
    }
}
