import ByzCoinRPC from "../byzcoin-rpc";
import Darc from "../../darc/darc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import Signer from "../../darc/signer";
import Instance from "../instance";

export default class DarcInstance {
    static readonly contractID = "darc";

    private darc: Darc;
    private rpc: ByzCoinRPC;

    constructor(rpc: ByzCoinRPC, instance: Instance) {
        if (instance.contractID.toString() !== DarcInstance.contractID) {
            throw new Error(`mismatch contract name: ${instance.contractID} vs ${DarcInstance.contractID}`);
        }

        this.rpc = rpc;
        this.darc = Darc.decode(instance.data);
    }

    /**
     * Get the darc of the instance
     * @returns the darc
     */
    getDarc(): Darc {
        return this.darc;
    }

    /**
     * Update the data of this instance
     *
     * @return a promise that resolves once the data is up-to-date
     */
    async update(): Promise<DarcInstance> {
        const proof = await this.rpc.getProof(this.darc.baseID);
        this.darc = Darc.fromProof(this.darc.baseID, proof);

        return this;
    }

    /**
     * Request to evolve the existing darc using the new darc and wait for
     * the block inclusion
     * 
     * @param newDarc The new darc
     * @param signers Signers for the counters
     * @param wait Number of blocks to wait for
     * @returns a promise that resolves with the new darc instance
     */
    async evolveDarcAndWait(newDarc: Darc, signers: Signer[], wait: number): Promise<DarcInstance> {
        const args = [new Argument({ name: 'darc', value: Buffer.from(Darc.encode(newDarc).finish()) })];
        const instr = Instruction.createInvoke(this.darc.baseID, DarcInstance.contractID, 'evolve', args);

        const ctx = new ClientTransaction({ instructions: [instr] });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx, wait);

        return DarcInstance.fromByzcoin(this.rpc, this.darc.baseID);
    }

    /**
     * Request to spawn an instance and wait for the inclusion
     * 
     * @param contractID    Contract name of the new instance
     * @param signers       Signers for the counters
     * @param wait          Number of blocks to wait for
     * @returns a promise that resolves with the new darc instance
     */
    async spawnDarcAndWait(d: Darc, signers: Signer[], wait: number = 0): Promise<DarcInstance> {
        const args = [
            new Argument({
                name: 'darc',
                value: Buffer.from(Darc.encode(d).finish()),
            }),
        ];
        const instr = Instruction.createSpawn(this.darc.baseID, DarcInstance.contractID, args);

        const ctx = new ClientTransaction({ instructions: [instr] });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx, wait);

        return DarcInstance.fromByzcoin(this.rpc, d.baseID);
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
