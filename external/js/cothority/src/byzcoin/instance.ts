import Proof from "./proof";
import ByzCoinRPC from "./byzcoin-rpc";

export default class Instance {
    protected constructor(
        readonly id: Buffer,
        readonly contractID: Buffer,
        readonly darcID: Buffer,
        readonly data: Buffer,
    ) {}

    public static fromProof(p: Proof): Instance {
        return new Instance(p.key, p.contractID, p.darcID, p.value);
    }

    public static async fromByzCoin(rpc: ByzCoinRPC, id: Buffer): Promise<Instance> {
        const p = await rpc.getProof(id);
        if (!p || !p.exists(id)) {
            throw new Error('instance is not in proof');
        }

        return Instance.fromProof(p);
    }
}