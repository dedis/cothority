import { Argument, ByzCoinRPC, ClientTransaction, InstanceID, Instruction } from "../../byzcoin";
import { AddTxResponse } from "../../byzcoin/proto/requests";
import ISigner from "../../darc/signer";

/**
 * TransactionBuilder handles collecting multiple instructions and signing them all
 * together before sending the transaction to the chain.
 * There are convenience methods to create spawn, invoke, or delete instructions.
 * Once all instructions are added, the send method will contact one or more nodes
 * to submit the transaction.
 * After a call to the send method, the transaction is ready for new instructions.
 *
 * If any of the instructions in this transaction fails, all other instructions will
 * fail, too.
 */
export class TransactionBuilder {
    private instructions: Instruction[] = [];

    constructor(protected bc: ByzCoinRPC) {
    }

    /**
     * Signs all instructions and sends them to the nodes.
     * The `instructions` field is only emptied if the transaction has been accepted successfully.
     * If the transaction fails, it can be retried.
     *
     * @param signers one set of signers per instruction. If there is only one set for multiple
     * instructions, always the same set of signers will be used.
     * @param wait if 0, doesn't wait for inclusion. If > 0, waits for inclusion for this many blockIntervals.
     */
    async send(signers: ISigner[][], wait = 0): Promise<[ClientTransaction, AddTxResponse]> {
        const ctx = ClientTransaction.make(this.bc.getProtocolVersion(), ...this.instructions);
        await ctx.updateCountersAndSign(this.bc, signers);
        const response = await this.bc.sendTransactionAndWait(ctx, wait);
        this.instructions = [];
        return [ctx, response];
    }

    /**
     * @return true if one or more instructions are available.
     */
    hasInstructions(): boolean {
        return this.instructions.length > 0;
    }

    /**
     * Appends a new instruction.
     *
     * @param inst the new instruction to append
     * @return the appended instruction
     */
    append(inst: Instruction): Instruction {
        this.instructions.push(inst);
        return inst;
    }

    /**
     * Prepends a new instruction
     *
     * @param inst the instruction to prepend
     * @return the prepended instruction
     */
    prepend(inst: Instruction): Instruction {
        this.instructions.unshift(inst);
        return inst;
    }

    /**
     * Appends a spawn instruction.
     *
     * @param iid the instance ID where the instruction is sent to
     * @param contractID the contractID to spawn
     * @param args arguments of the contract
     * @return new instruction - can be used for deriveID
     */
    spawn(iid: Buffer, contractID: string, args: Argument[]): Instruction {
        return this.append(Instruction.createSpawn(iid, contractID, args));
    }

    /**
     * Appends an invoke instruction.
     *
     * @param iid the instance ID where the instruction is sent to
     * @param contractID the contractID to invoke
     * @param command to be invoked on the contract
     * @param args arguments of the command
     * @return new instruction - can be used for deriveID
     */
    invoke(iid: Buffer, contractID: string, command: string, args: Argument[]): Instruction {
        return this.append(Instruction.createInvoke(iid, contractID, command, args));
    }

    /**
     * Appends a delete instruction.
     *
     * @param iid the instance ID where the instruction is sent to
     * @param contractID to be deleted - must match the actual contract
     * @return new instruction - can be used for deriveID
     */
    delete(iid: Buffer, contractID: string): Instruction {
        return this.append(Instruction.createDelete(iid, contractID));
    }

    /**
     * Returns the ID that will be produced by the given instruction
     *
     * @param index of the instruction
     * @param name if given, will be passed to deriveID
     */
    deriveID(index: number, name = ""): InstanceID {
        if (index < 0 || index > this.instructions.length) {
            throw new Error("instruction out of bound");
        }
        return this.instructions[index].deriveId(name);
    }

    /**
     * returns a useful and readable representation of all instructions in this transaction.
     */
    toString(): string {
        return this.instructions.map((inst, i) => {
            const t = ["Spawn", "Invoke", "Delete"][inst.type];
            let cid: string;
            let args: Argument[];
            switch (inst.type) {
                case Instruction.typeSpawn:
                    cid = inst.spawn.contractID;
                    args = inst.spawn.args;
                    break;
                case Instruction.typeInvoke:
                    cid = `${inst.invoke.contractID} / ${inst.invoke.command}`;
                    args = inst.invoke.args;
                    break;
                case Instruction.typeDelete:
                    cid = inst.delete.contractID;
                    args = [];
                    break;
            }
            return `${i}:  ${t} ${cid}: ${inst.instanceID.toString("hex")}\n\t` +
                args.map((kv) => `${kv.name}: ${kv.value.toString("hex")}`).join("\n\t");
        }).join("\n\n");
    }
}
