import * as Long from "long";

const crypto = require("crypto-browserify");

import {Signer} from "~/lib/cothority/darc/Signer";
import {Signature} from "~/lib/cothority/darc/Signature";
import {ByzCoinRPC} from "~/lib/cothority/byzcoin/ByzCoinRPC";
import {Log} from "~/lib/Log";

export class ClientTransaction {
    instructions: Instruction[];

    constructor(inst: Instruction[]) {
        this.instructions = inst;
    }

    async signBy(ss: Signer[][], bc: ByzCoinRPC = null) {
        try {
            if (bc != null) {
                Log.lvl3("Updating signature counters");
                for (let i = 0; i < ss.length; i++) {
                    let ids = ss[i].map(s => s.identity);
                    this.instructions[i].signerCounter =
                        await bc.getSignerCounters(ids, 1);
                }
            }
            let ctxHash = this.hash();
            ss.forEach((signers, i) => {
                this.instructions[i].signatures =
                    signers.map(signer => {
                        return signer.sign(ctxHash);
                    })
            })
        } catch (e) {
            Log.catch(e);
        }
    }

    hash(): Buffer {
        let h = crypto.createHash("sha256");
        this.instructions.forEach(i => h.update(i.hash()));
        return h.digest();
    }

    toObject(): object {
        return {
            instructions: this.instructions.map(inst => inst.toObject()),
        };
    }
}

export class Instruction {
    spawn: Spawn;
    invoke: Invoke;
    delete: Delete;

    constructor(public instanceID: InstanceID,
                public signerCounter: Long[] = [],
                public signatures: Signature[] = []) {
    }

    toObject(): object {
        return {
            instanceid: this.instanceID.iid,
            spawn: this.spawn ? this.spawn.toObject() : null,
            invoke: this.invoke ? this.invoke.toObject() : null,
            delete: this.delete ? this.delete.toObject() : null,
            signercounter: this.signerCounter,
            signatures: this.signatures.map(sig => sig.toObject()),
        }
    }

    hash(): Buffer {
        let h = crypto.createHash("sha256");
        h.update(this.instanceID.iid);
        h.update(Buffer.from([this.getType()]));
        let args: Argument[] = [];
        switch (this.getType()) {
            case 0:
                h.update(Buffer.from(this.spawn.contractID));
                args = this.spawn.args;
                break;
            case 1:
                args = this.invoke.args;
                break;
        }
        args.forEach(arg => {
            h.update(arg.name);
            h.update(arg.value)
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
        let h = crypto.createHash("sha256");
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

    static createSpawn(iid: InstanceID, contractID: string, args: Argument[]): Instruction {
        let inst = new Instruction(iid);
        inst.spawn = new Spawn(contractID, args);
        return inst;
    }

    static createInvoke(iid: InstanceID, cmd: string, args: Argument[]): Instruction {
        let inst = new Instruction(iid);
        inst.invoke = new Invoke(cmd, args);
        return inst;
    }

    static createDelete(iid: InstanceID): Instruction {
        let inst = new Instruction(iid);
        inst.delete = new Delete();
        return inst;
    }
}

export class Argument {
    name: string;
    value: Buffer;

    toObject(): object {
        return {
            name: this.name,
            value: this.value,
        };
    }

    constructor(n: string, v: Buffer) {
        this.name = n;
        this.value = v;
    }
}

export class Spawn {
    constructor(public contractID: string, public args: Argument[]) {
    }

    toObject(): object {
        return {
            contractid: this.contractID,
            args: this.args.map(arg => arg.toObject()),
        };
    }
}

export class Invoke {
    constructor(public command: string, public args: Argument[]) {
    }

    toObject(): object {
        return this;
    }
}

export class Delete {
    toObject(): object {
        return {};
    }
}

export class InstanceID {
    iid: Buffer;

    constructor(iid: Buffer) {
        if (iid.length != 32) {
            throw new Error("instanceIDs are always 32 bytes");
        }
        this.iid = Buffer.from(iid);
    }

    equals(iid: InstanceID) {
        return this.iid.equals(iid.iid);
    }

    toObject(): any {
        return {IID: this.iid};
    }

    static fromHex(str: string): InstanceID {
        return new InstanceID(Buffer.from(str, 'hex'));
    }

    static fromObject(obj: any): InstanceID {
        return new InstanceID(Buffer.from(obj.IID));
    }

    static fromObjectBuffer(obj: any): InstanceID {
        return new InstanceID(Buffer.from(obj));
    }
}