import Signer from "../darc/signer";
import Signature from "../darc/signature";
import { createHash } from "crypto";
import { Message } from "protobufjs";
import Long from 'long';
import ByzCoinRPC from "./byzcoin-rpc";
import Identity from "../darc/identity";

export default class ClientTransaction extends Message<ClientTransaction> {
    readonly instructions: Instruction[];

    signWith(signers: Signer[]) {
        const ctxHash = this.hash();

        this.instructions.forEach((instr) => instr.signWith(ctxHash, signers));
    }

    async updateCounters(rpc: ByzCoinRPC, signers: Identity[]): Promise<void> {
        if (this.instructions.length === 0) {
            return;
        }

        await this.instructions[0].updateCounters(rpc, signers);

        for (let i = 1; i < this.instructions.length; i++) {
            this.instructions[i].signerCounter = this.instructions[0].signerCounter.map(v => v.add(i));
        }
    }

    hash(): Buffer {
        let h = createHash("sha256");
        this.instructions.forEach(i => h.update(i.hash()));
        return h.digest();
    }
}

export class Instruction extends Message<Instruction> {
    private instanceid: Buffer;
    readonly spawn: Spawn;
    readonly invoke: Invoke;
    readonly delete: Delete;
    private signercounter: Long[];

    private _signatures: Signature[];

    get instanceID(): Buffer {
        return this.instanceid;
    }

    set instanceID(v: Buffer) {
        this.instanceid = v;
    }

    get signerCounter(): Long[] {
        return this.signercounter;
    }

    set signerCounter(v: Long[]) {
        this.signercounter = v;
    }

    get signatures(): Signature[] {
        // readonly access with internal modification via signWith
        return this._signatures;
    }

    signWith(ctxHash: Buffer, signers: Signer[]): void {
        this._signatures = signers.map(s => s.sign(ctxHash));
    }

    async updateCounters(rpc: ByzCoinRPC, signers: Identity[]): Promise<void> {
        const counters = await rpc.getSignerCounters(signers, 1);
        this.signerCounter = counters;
    }

    hash(): Buffer {
        let h = createHash("sha256");
        h.update(this.instanceid);
        h.update(Buffer.from([this.getType()]));
        let args: Argument[] = [];
        switch (this.getType()) {
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
        args.forEach(arg => {
            h.update(arg.name);
            h.update(arg.value);
        });
        this.signerCounter.forEach(sc => {
            h.update(Buffer.from(sc.toBytesLE()));
        });
        return h.digest();
    }

    getType(): number {
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

    deriveId(what: string = ""): Buffer {
        let h = createHash("sha256");
        h.update(this.hash());
        let b = Buffer.alloc(4);
        b.writeUInt32LE(this.signatures.length, 0);
        h.update(b);
        this.signatures.forEach(sig => {
            b.writeUInt32LE(sig.signature.length, 0);
            h.update(b);
            h.update(sig.signature);
        });
        h.update(Buffer.from(what));
        return h.digest();
    }

    static createSpawn(iid: Buffer, contractID: string, args: Argument[]): Instruction {
        return new Instruction({
            instanceID: iid,
            spawn: new Spawn({ contractID, args }),
            signerCounter: [],
        });
    }

    static createInvoke(iid: Buffer, contractID: string, command: string, args: Argument[]): Instruction {
        return new Instruction({
            instanceID: iid,
            invoke: new Invoke({ command, contractID, args }),
            signerCounter: [],
        });
    }

    static createDelete(iid: Buffer, contractID: string): Instruction {
        return new Instruction({
            instanceID: iid,
            delete: new Delete({ contractID }),
            signerCounter: [],
        });
    }
}

export class Argument extends Message<Argument> {
    readonly name: string;
    readonly value: Buffer;
}

export class Spawn extends Message<Spawn> {
    private contractid: string;
    readonly args: Argument[];

    get contractID(): string {
        return this.contractid;
    }

    set contractID(v: string) {
        this.contractid = v;
    }
}

export class Invoke extends Message<Invoke> {
    private contractid: string;
    readonly command: string;
    readonly args: Argument[];

    get contractID(): string {
        return this.contractid;
    }

    set contractID(v: string) {
        this.contractid = v;
    }
}

export class Delete extends Message<Delete> {
    private contractid: string;

    get contractID(): string {
        return this.contractid;
    }

    set contractID(v: string) {
        this.contractid = v;
    }
}
