import { createHash } from "crypto-browserify";
import _ from "lodash";
import Long from "long";
import { Message, Properties } from "protobufjs/light";
import IdentityWrapper, { IIdentity } from "../darc/identity-wrapper";
import Signer from "../darc/signer";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import { InstanceID } from "./instance";

export interface ICounterUpdater {
    getSignerCounters(signers: IIdentity[], increment: number): Promise<Long[]>;
    updateCachedCounters(signers: IIdentity[]): Promise<Long[]>;
    getNextCounter(signer: IIdentity): Long;
}

/**
 * List of instructions to send to a byzcoin chain
 */
export default class ClientTransaction extends Message<ClientTransaction> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.ClientTransaction", ClientTransaction, Instruction);
    }

    /**
     * Create a transaction from the list of instructions.
     * @param version   Version of the ByzCoin protocol
     * @param instrs    List of instructions
     */
    static make(version: number, ...instrs: Instruction[]): ClientTransaction {
        if (version >= 1) {
            instrs = instrs.map((i) => new InstructionV1(i));
        }

        return new ClientTransaction({ instructions: instrs });
    }

    readonly instructions: Instruction[];

    /**
     * @deprecated Use the static function make to create transactions compatible with
     * latest versions of the ByzCoin protocol.
     */
    constructor(props?: Properties<ClientTransaction>) {
        super(props);

        this.instructions = this.instructions || [];
    }

    /**
     * Sign the hash of the instructions using the list of signers
     * @param signers List of signers
     */
    signWith(signers: Signer[][]): void {
        const ctxHash = this.hash();

        if (signers.length !== this.instructions.length) {
            throw new Error("need same number of signers as instructions");
        }

        this.instructions.forEach((instr, i) => instr.signWith(ctxHash, signers[i]));
    }

    /**
     * Fetch the counters and update the instructions accordingly
     * @param rpc       The RPC to use to fetch
     * @param signers   List of signers
     */
    async updateCounters(rpc: ICounterUpdater, signers: IIdentity[][]): Promise<void> {
        if (this.instructions.length === 0) {
            return;
        }

        if (signers.length !== this.instructions.length) {
            return Promise.reject("length of signers and instructions do not match");
        }

        // Get all counters from all signers of all instructions and map them into an object.
        const uniqueSigners: IIdentity[] = _.uniq(_.flatten(signers));
        await rpc.updateCachedCounters(uniqueSigners);

        // Iterate over the instructions, and store the appropriate signers and counters, while
        // increasing those that have been used.
        for (let i = 0; i < this.instructions.length; i++) {
            signers[i].forEach((signer) => {
                this.instructions[i].signerIdentities.push(IdentityWrapper.fromIdentity(signer));
                this.instructions[i].signerCounter.push(rpc.getNextCounter(signer));
            });
        }
    }

    async updateCountersAndSign(rpc: ICounterUpdater, signers: Signer[][]): Promise<void> {
        // Extend the signers to the number of instructions if there is only one signer.
        if (signers.length === 1) {
            for (let i = 1; i < this.instructions.length; i++) {
                signers.push(signers[0]);
            }
        }
        await this.updateCounters(rpc, signers);
        this.signWith(signers);
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
    static readonly typeSpawn = 0;
    static readonly typeInvoke = 1;
    static readonly typeDelete = 2;

    /**
     * Get the type of the instruction
     * @returns the type as a number
     */
    get type(): number {
        if (this.spawn) {
            return Instruction.typeSpawn;
        }
        if (this.invoke) {
            return Instruction.typeInvoke;
        }
        if (this.delete) {
            return Instruction.typeDelete;
        }
        throw new Error("instruction without type");
    }
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.Instruction", Instruction, IdentityWrapper, Spawn, Invoke, Delete);
    }

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
            spawn: new Spawn({contractID, args}),
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
            invoke: new Invoke({command, contractID, args}),
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
            delete: new Delete({contractID}),
            instanceID: iid,
            signerCounter: [],
        });
    }

    readonly spawn: Spawn;
    readonly invoke: Invoke;
    readonly delete: Delete;
    readonly instanceID: InstanceID;
    readonly signerCounter: Long[];
    readonly signerIdentities: IdentityWrapper[];
    readonly signatures: Buffer[];

    constructor(props?: Properties<Instruction>) {
        super(props);

        this.signerCounter = this.signerCounter || [];
        this.signerIdentities = this.signerIdentities || [];
        this.signatures = this.signatures || [];

        /* Protobuf aliases */

        Object.defineProperty(this, "instanceid", {
            get(): InstanceID {
                return this.instanceID;
            },
            set(value: InstanceID) {
                this.instanceID = value;
            },
        });

        Object.defineProperty(this, "signercounter", {
            get(): Long[] {
                return this.signerCounter;
            },
            set(value: Long[]) {
                this.signerCounter = value;
            },
        });

        Object.defineProperty(this, "signeridentities", {
            get(): IdentityWrapper[] {
                return this.signerIdentities;
            },
            set(value: IdentityWrapper[]) {
                this.signerIdentities = value;
            },
        });
    }

    /**
     * Use the signers to make a signature of the hash
     * @param ctxHash The client transaction hash
     * @param signers The list of signers
     */
    signWith(ctxHash: Buffer, signers: Signer[]): void {
        // @ts-ignore
        this.signatures = signers.map((s) => s.sign(ctxHash));
    }

    /**
     * Set the signer counters and identities
     * @param counters      List of counters
     * @param identities    List of identities
     */
    setCounters(counters: Long[], identities: IdentityWrapper[]): void {
        // @ts-ignore
        this.signerCounter = counters;

        // @ts-ignore
        this.signerIdentities = identities;
    }

    /**
     * Fetch and update the counters
     * @param rpc       the RPC to use to fetch
     * @param signers   the list of signers
     */
    async updateCounters(rpc: ICounterUpdater, signers: IIdentity[]): Promise<void> {
        const counters = await rpc.getSignerCounters(signers, 1);

        this.setCounters(counters, signers.map((s) => IdentityWrapper.fromIdentity(s)));
    }

    /**
     * Hash the instruction
     * @returns a buffer of the hash
     */
    hash(): Buffer {
        return this.hashForVersion(0);
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

    protected hashForVersion(version: number): Buffer {
        const h = createHash("sha256");
        h.update(this.instanceID);
        h.update(Buffer.from([this.type]));
        let args: Argument[] = [];
        switch (this.type) {
            case Instruction.typeSpawn:
                h.update(this.spawn.contractID);
                args = this.spawn.args;
                break;
            case Instruction.typeInvoke:
                h.update(this.invoke.contractID);
                if (version >= 1) {
                    h.update(this.invoke.command);
                }
                args = this.invoke.args;
                break;
            case Instruction.typeDelete:
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
}

/**
 * Extension of the initial version of an instruction to include the
 * invoke command in the hash.
 * Use ClientTransaction.make in order to create compatible transactions.
 */
class InstructionV1 extends Instruction {
    hash(): Buffer {
        return this.hashForVersion(1);
    }
}

/**
 * Argument of an instruction
 */
export class Argument extends Message<Argument> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.Argument", Argument);
    }

    readonly name: string;
    readonly value: Buffer;

    constructor(props?: Properties<Argument>) {
        super(props);

        this.value = Buffer.from(this.value || EMPTY_BUFFER);
    }
}

/**
 * Spawn instruction that will create instances
 */
export class Spawn extends Message<Spawn> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.Spawn", Spawn, Argument);
    }

    readonly args: Argument[];
    readonly contractID: string;

    constructor(props?: Properties<Spawn>) {
        super(props);

        this.args = this.args || [];

        /* Protobuf aliases */

        Object.defineProperty(this, "contractid", {
            get(): string {
                return this.contractID;
            },
            set(value: string) {
                this.contractID = value;
            },
        });
    }
}

/**
 * Invoke instruction that will update an existing instance
 */
export class Invoke extends Message<Invoke> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.Invoke", Invoke, Argument);
    }

    readonly command: string;
    readonly args: Argument[];
    readonly contractID: string;

    constructor(props?: Properties<Invoke>) {
        super(props);

        this.args = this.args || [];

        /* Protobuf aliases */

        Object.defineProperty(this, "contractid", {
            get(): string {
                return this.contractID;
            },
            set(value: string) {
                this.contractID = value;
            },
        });
    }
}

/**
 * Delete instruction that will delete an instance
 */
export class Delete extends Message<Delete> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.Delete", Delete);
    }

    readonly contractID: string;

    constructor(props?: Properties<Delete>) {
        super(props);

        Object.defineProperty(this, "contractid", {
            get(): string {
                return this.contractID;
            },
            set(value: string) {
                this.contractID = value;
            },
        });
    }
}

ClientTransaction.register();
Instruction.register();
Argument.register();
Spawn.register();
Invoke.register();
Delete.register();
