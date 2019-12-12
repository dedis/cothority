import Signer from "../../darc/signer";
import ByzCoinRPC from "../byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import Instance, { InstanceID } from "../instance";

export default class ValueInstance extends Instance {
    static readonly contractID = "value";
    static readonly commandUpdate = "update";
    static readonly argumentValue = "value";

    /**
     * Spawn a value instance from a darc id
     *
     * @param bc        The RPC to use
     * @param darcID    The darc instance ID
     * @param signers   The list of signers for the transaction
     * @param value     The value to be put in the value instance
     * @returns a promise that resolves with the new instance
     */
    static async spawn(
        bc: ByzCoinRPC,
        darcID: InstanceID,
        signers: Signer[],
        value: Buffer,
    ): Promise<ValueInstance> {
        const inst = Instruction.createSpawn(
            darcID,
            ValueInstance.contractID,
            [new Argument({name: ValueInstance.argumentValue, value})],
        );
        await inst.updateCounters(bc, signers);

        const ctx = ClientTransaction.make(bc.getProtocolVersion(), inst);
        ctx.signWith([signers]);

        await bc.sendTransactionAndWait(ctx, 10);

        return ValueInstance.fromByzcoin(bc, ctx.instructions[0].deriveId(), 1);
    }

    /**
     * Create returns a ValueInstance from the given parameters.
     * @param bc
     * @param valueID
     * @param darcID
     * @param value
     */
    static create(
        bc: ByzCoinRPC,
        valueID: InstanceID,
        darcID: InstanceID,
        value: Buffer,
    ): ValueInstance {
        return new ValueInstance(bc, new Instance({
            contractID: ValueInstance.contractID,
            darcID,
            data: value,
            id: valueID,
        }));
    }

    /**
     * Initializes using an existing ValueInstance from ByzCoin
     * @param bc    The RPC to use
     * @param iid   The instance ID
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @returns a promise that resolves with the coin instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<ValueInstance> {
        return new ValueInstance(bc, await Instance.fromByzcoin(bc, iid, waitMatch, interval));
    }

    _value: Buffer;

    /**
     * @return value of the instance
     */
    get value(): Buffer {
        return this._value;
    }

    /**
     * Constructs a new ValueInstance. If the instance is not of type ValueInstance,
     * an error will be thrown.
     *
     * @param rpc a working RPC instance
     * @param inst an instance representing a ValueInstance
     */
    constructor(private rpc: ByzCoinRPC, inst: Instance) {
        super(inst);
        if (inst.contractID.toString() !== ValueInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${ValueInstance.contractID}`);
        }

        this._value = this.data;
    }

    /**
     * Update the value
     *
     * @param signers   The list of signers for the transaction
     * @param value     The new value to be set
     * @param wait      Number of blocks to wait for inclusion
     */
    async updateValue(signers: Signer[], value: Buffer, wait?: number): Promise<void> {
        const inst = Instruction.createInvoke(
            this.id,
            ValueInstance.contractID,
            ValueInstance.commandUpdate,
            [new Argument({name: ValueInstance.argumentValue, value})],
        );
        await inst.updateCounters(this.rpc, signers);

        const ctx = ClientTransaction.make(this.rpc.getProtocolVersion(), inst);
        ctx.signWith([signers]);

        await this.rpc.sendTransactionAndWait(ctx, wait);
    }

    /**
     * Update the data of this instance
     *
     * @returns the updated instance
     */
    async update(): Promise<ValueInstance> {
        const p = await this.rpc.getProofFromLatest(this.id);
        if (!p.exists(this.id)) {
            throw new Error("fail to get a matching proof");
        }

        this._value = p.value;
        return this;
    }
}
