import ByzCoinRPC from "../byzcoin-rpc";
import Darc from "../../darc/darc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import Proof from "../proof";
import Signer from "../../darc/signer";
import Instance from "../instance";

export default class DarcInstance {
    static readonly contractID = "darc";

    private instance: Instance;
    public darc: Darc;
    private rpc: ByzCoinRPC;

    constructor(rpc: ByzCoinRPC, instance: Instance) {
        if (instance.contractID.toString() !== DarcInstance.contractID) {
            throw new Error(`mismatch contract name: ${instance.contractID} vs ${DarcInstance.contractID}`);
        }

        this.rpc = rpc;
        this.instance = instance;
        this.darc = Darc.decode(instance.data);
    }

    /**
     * Update the data of this instance
     *
     * @return {Promise<DarcInstance>} - a promise that resolves once the data
     * is up-to-date
     */
    async update(): Promise<DarcInstance> {
        const proof = await this.rpc.getProof(this.darc.baseID);
        this.darc = Darc.fromProof(this.darc.baseID, proof);

        return this;
    }

    async evolveDarcAndWait(newDarc: Darc, signer: Signer, wait: number): Promise<Proof> {
        const args = [new Argument({ name: 'darc', value: Buffer.from(Darc.encode(newDarc).finish()) })];
        const instr = Instruction.createInvoke(this.darc.baseID, DarcInstance.contractID, 'evolve', args);
        const ctx = new ClientTransaction({ instructions: [instr] });

        await instr.updateCounters(this.rpc, [signer]);

        ctx.signWith([signer]);

        await this.rpc.sendTransactionAndWait(ctx, wait);

        const proof = await this.rpc.getProof(this.darc.baseID);
        if (!proof.exists(this.darc.baseID)) {
            throw new Error('instance is not in proof');
        }

        return proof;
    }

    async spawnInstanceAndWait(contractID: string, signer: Signer, args: Argument[], wait: number): Promise<DarcInstance> {
        const instr = Instruction.createSpawn(this.darc.baseID, DarcInstance.contractID, args);
        const ctx = new ClientTransaction({ instructions: [instr] });

        // Get the counters before the signature
        const counters = await this.rpc.getSignerCounters([signer], 1);
        instr.signerCounter = counters;

        ctx.signWith([signer]);

        await this.rpc.sendTransactionAndWait(ctx, wait);

        let iid = instr.deriveId();
        if (contractID === DarcInstance.contractID) {
            const d = Darc.decode(args[0].value);
            iid = d.baseID;
        }

        return DarcInstance.fromByzcoin(this.rpc, iid);
    }

    spawnDarcAndWait(d: Darc, signer: Signer, wait: number = 0): Promise<DarcInstance> {
        const args = [new Argument({ name: 'darc', value: Buffer.from(Darc.encode(d).finish()) })];

        return this.spawnInstanceAndWait(DarcInstance.contractID, signer, args, wait);
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * @param bc
     * @param instID
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: Buffer): Promise<DarcInstance> {
        return new DarcInstance(bc, await Instance.fromByzCoin(bc, iid));
    }
}
