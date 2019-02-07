import ByzCoinRPC from "./byzcoin-rpc";
import Proof from "./proof";

export type InstanceID = Buffer;

/**
 * Instance with basic information
 */
export default class Instance {
    /**
     * Create an instance from a proof
     * @param p The proof
     * @returns the instance
     */
    static fromProof(key: InstanceID, p: Proof): Instance {
        if (!p.exists(key)) {
            throw new Error(`key not in proof: ${key.toString("hex")}`);
        }

        return new Instance(key, p.contractID, p.darcID, p.value);
    }

    /**
     * Create an instance after requesting its proof to byzcoin
     * @param rpc   The RPC to use
     * @param id    The ID of the instance
     * @returns the instance if it exists
     */
    static async fromByzCoin(rpc: ByzCoinRPC, id: InstanceID): Promise<Instance> {
        const p = await rpc.getProof(id);

        return Instance.fromProof(id, p);
    }

    readonly id: InstanceID;
    readonly contractID: Buffer;
    readonly darcID: InstanceID;
    readonly data: Buffer;

    protected constructor(id: Buffer, contractID: Buffer, darcID: Buffer, data: Buffer) {
        this.id = id;
        this.contractID = contractID;
        this.darcID = darcID;
        this.data = data;
    }
}
