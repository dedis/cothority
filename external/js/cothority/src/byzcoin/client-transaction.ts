import { createHash } from "crypto";
import Long from "long";
import { Message } from "protobufjs/light";
import IdentityWrapper, { IIdentity } from "../darc/identity-wrapper";
import Signer from "../darc/signer";

export interface ICounterUpdater {
    getSignerCounters(signers: IIdentity[], increment: number): Promise<Long[]>;
}

/**
 * List of instructions to send to a byzcoin chain
 */
export default class ClientTransaction extends Message<ClientTransaction> {
    readonly instructions: Instruction[];

    /**
     * Sign the hash of the instructions using the list of signers
     * @param signers List of signers
     */
    signWith(signers: Signer[]): void {
        const ctxHash = this.hash();

        this.instructions.forEach((instr) => instr.signWith(ctxHash, signers));
    }

    /**
     * Fetch the counters and update the instructions accordingly
     * @param rpc       The RPC to use to fetch
     * @param signers   List of signers
     */
    async updateCounters(rpc: ICounterUpdater, signers: IIdentity[]): Promise<void> {
        if (this.instructions.length === 0) {
            return;
        }

        await this.instructions[0].updateCounters(rpc, signers);

        for (let i = 1; i < this.instructions.length; i++) {
            this.instructions[i].signerCounter = this.instructions[0].signerCounter.map((v) => v.add(i));
            this.instructions[i].signerIdentities = signers.map((s) => s.toWrapper());
        }
    }

    /**
     * Hash the instructions' hash
     * @returns a buffer of the hash
     */
    hash(): Buffer {
        const h = createHash("sha256");
        this.instructions.forEach((i) => h.update(i.hash()));
        return h.digest();
    }
}

/**
 * An instruction represents one action
 */
export class Instruction extends Message<Instruction> {
    /**
     * Helper to create a spawn instruction
     * @param iid           The instance ID
     * @param contractID    The contract name
     * @param args          Arguments for the instruction
     * @returns the instruction
     */
    static createSpawn(iid: Buffer, contractID: string, args: Argument[]): Instruction {
        return new Instruction({
            instanceID: iid,
            signerCounter: [],
            spawn: new Spawn({ contractID, args }),
        });
    }

    /**
     * Helper to create a invoke instruction
     * @param iid           The instance ID
     * @param contractID    The contract name
     * @param command       The command to invoke
     * @param args          The list of arguments
     * @returns the instruction
     */
    static createInvoke(iid: Buffer, contractID: string, command: string, args: Argument[]): Instruction {
        return new Instruction({
            instanceID: iid,
            invoke: new Invoke({ command, contractID, args }),
            signerCounter: [],
        });
    }

    /**
     * Helper to create a delete instruction
     * @param iid           The instance ID
     * @param contractID    The contract name
     * @returns the instruction
     */
    static createDelete(iid: Buffer, contractID: string): Instruction {
        return new Instruction({
            delete: new Delete({ contractID }),
            instanceID: iid,
            signerCounter: [],
        });
    }
    readonly spawn: Spawn;
    readonly invoke: Invoke;
    readonly delete: Delete;
    private instanceid: Buffer;
    private signercounter: Long[];
    private signeridentities: IdentityWrapper[];

    private _signatures: Buffer[];

    /**
     * Getter for the instance ID
     * @returns the instance ID
     */
    get instanceID(): Buffer {
        return this.instanceid;
    }

    /**
     * Setter for the instance ID
     * @param v The value to set
     */
    set instanceID(v: Buffer) {
        this.instanceid = v;
    }

    /**
     * Getter for the signer counters
     * @returns the counters
     */
    get signerCounter(): Long[] {
        return this.signercounter;
    }

    /**
     * Setter for the signer counters
     * @param v The value to set
     */
    set signerCounter(v: Long[]) {
        this.signercounter = v;
    }

    /**
     * Getter for the signer identites
     * @returns the signer identities
     */
    get signerIdentities(): IdentityWrapper[] {
        return this.signeridentities;
    }

    /**
     * Setter for the signer identities
     * @param v The list of identity wrappers
     */
    set signerIdentities(v: IdentityWrapper[]) {
        this.signeridentities = v;
    }

    /**
     * Getter for the signatures
     * @returns the list of signatures
     */
    get signatures(): Buffer[] {
        // readonly access with internal modification via signWith
        return this._signatures;
    }

    /**
     * Get the type of the instruction
     * @returns the type as a number
     */
    get type(): number {
        if (this.spawn) {
            return 0;
        }
        if (this.invoke) {
            return 1;
        }
        if (this.delete) {
            return 2;
        }
        throw new Error("instruction without type");
    }

    /**
     * Use the signers to make a signature of the hash
     * @param ctxHash The client transaction hash
     * @param signers The list of signers
     */
    signWith(ctxHash: Buffer, signers: Signer[]): void {
        this._signatures = signers.map((s) => s.sign(ctxHash));
    }

    /**
     * Fetch and update the counters
     * @param rpc       the RPC to use to fetch
     * @param signers   the list of signers
     */
    async updateCounters(rpc: ICounterUpdater, signers: IIdentity[]): Promise<void> {
        const counters = await rpc.getSignerCounters(signers, 1);
        this.signerCounter = counters;
        this.signeridentities = signers.map((s) => s.toWrapper());
    }

    /**
     * Hash the instruction
     * @returns a buffer of the hash
     */
    hash(): Buffer {
        const h = createHash("sha256");
        h.update(this.instanceid);
        h.update(Buffer.from([this.type]));
        let args: Argument[] = [];
        switch (this.type) {
            case 0:
                h.update(this.spawn.contractID);
                args = this.spawn.args;
                break;
            case 1:
                h.update(this.invoke.contractID);
                args = this.invoke.args;
                break;
            case 2:
                h.update(this.delete.contractID);
                break;
        }
        args.forEach((arg) => {
            const nameBuf = Buffer.from(arg.name);
            const nameLenBuf = Buffer.from(Long.fromNumber(nameBuf.length).toBytesLE());

            h.update(nameLenBuf);
            h.update(arg.name);

            const valueLenBuf = Buffer.from(Long.fromNumber(arg.value.length).toBytesLE());
            h.update(valueLenBuf);
            h.update(arg.value);
        });
        this.signerCounter.forEach((sc) => {
            h.update(Buffer.from(sc.toBytesLE()));
        });
        this.signerIdentities.forEach((si) => {
            const buf = si.toBytes();
            const lenBuf = Buffer.from(Long.fromNumber(buf.length).toBytesLE());

            h.update(lenBuf);
            h.update(si.toBytes());
        });
        return h.digest();
    }

    /**
     * Get the unique identifier of the instruction
     * @returns the id as a buffer
     */
    deriveId(what: string = ""): Buffer {
        const h = createHash("sha256");
        h.update(this.hash());
        const b = Buffer.alloc(4);
        b.writeUInt32LE(this.signatures.length, 0);
        h.update(b);
        this.signatures.forEach((sig) => {
            b.writeUInt32LE(sig.length, 0);
            h.update(b);
            h.update(sig);
        });
        h.update(Buffer.from(what));
        return h.digest();
    }
}

/**
 * Argument of an instruction
 */
export class Argument extends Message<Argument> {
    readonly name: string;
    readonly value: Buffer;
}

/**
 * Spawn instruction that will create instances
 */
export class Spawn extends Message<Spawn> {

    /**
     * Getter for the contract ID
     * @returns the contract ID
     */
    get contractID(): string {
        return this.contractid;
    }

    /**
     * Setter for the contract ID
     * @param v The value to set
     */
    set contractID(v: string) {
        this.contractid = v;
    }
    readonly args: Argument[];
    private contractid: string;
}

/**
 * Invoke instruction that will update an existing instance
 */
export class Invoke extends Message<Invoke> {

    /**
     * Getter for the contract ID
     * @returns the contract ID
     */
    get contractID(): string {
        return this.contractid;
    }

    /**
     * Setter for the contract ID
     * @param v The value to set
     */
    set contractID(v: string) {
        this.contractid = v;
    }
    readonly command: string;
    readonly args: Argument[];
    private contractid: string;
}

/**
 * Delete instruction that will delete an instance
 */
export class Delete extends Message<Delete> {
    private contractid: string;

    /**
     * Getter for the contract ID
     * @returns the contract ID
     */
    get contractID(): string {
        return this.contractid;
    }

    /**
     * Setter for the contract ID
     * @param v The value to set
     */
    set contractID(v: string) {
        this.contractid = v;
    }
}
