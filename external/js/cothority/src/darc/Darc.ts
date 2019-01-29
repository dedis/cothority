import { createHash } from 'crypto';
import {Identity} from "../darc/Identity";
import { Message, Properties } from "protobufjs";
import { registerMessage } from '../protobuf';
import { Proof } from '../byzcoin/Proof';
import { DarcInstance } from '../byzcoin/contracts/DarcInstance';
import Long from 'long';

export class Rule extends Message<Rule> {
    action: string;
    expr: Buffer;

    toString(): string {
        return this.action + " - " + this.expr.toString();
    }
}

export class Rules extends Message<Rules> {
    public static OR = '|';
    public static AND = '&';

    readonly list: Rule[];

    constructor(properties?: Properties<Rules>) {
        super(properties);

        if (!properties || !this.list) {
            this.list = [];
        }
    }

    appendToRule(action: string, identity: Identity, op: string): void {
        const rule = this.list.find(r => r.action === action);

        if (rule) {
            rule.expr = Buffer.concat([rule.expr, Buffer.from(` ${op} ${identity.toString()}`)]);
        } else {
            this.list.push(new Rule({ action, expr: Buffer.from(identity.toString()) }));
        }
    }

    toString(): string {
        return this.list.map(l => {
            return l.toString();
        }).join("\n");
    }
}

function initRules(owners: Identity[], signers: Identity[]): Rules {
    const rules = new Rules();

    owners.forEach((o) => rules.appendToRule('invoke:evolve', o, Rules.AND));
    signers.forEach(s => rules.appendToRule('_sign', s, Rules.OR));

    return rules;
}

export class Darc extends Message<Darc> {
    readonly version: Long;
    readonly description: Buffer;
    private baseid: Buffer;
    readonly previd: Buffer;
    readonly rules: Rules;

    get id(): Buffer {
        let h = createHash("sha256");
        let versionBuf = Buffer.from(this.version.toBytesLE());
        h.update(versionBuf);
        h.update(this.description);
        if (this.baseid) {
            h.update(this.baseid);
        }
        if (this.previd) {
            h.update(this.previd);
        }
        this.rules.list.forEach(r => {
            h.update(r.action);
            h.update(r.expr);
        });
        return h.digest();
    }

    get baseID(): Buffer {
        if (this.version.eq(0)) {
            return this.id;
        } else {
            return this.baseid;
        }
    }

    set baseID(id: Buffer) {
        this.baseid = id;
    }

    addIdentity(rule: string, identity: Identity, op: string): void {
        this.rules.appendToRule(rule, identity, op);
    }

    /**
     * Copy and evolve the darc to the next version
     */
    evolve(): Darc {
        return new Darc({
            version: this.version.add(1),
            description: this.description,
            baseID: this.baseID,
            previd: this.id,
            rules: this.rules,
        });
    }

    toString(): string {
        return "ID: " + this.id.toString('hex') + "\n" +
            "Base: " + this.baseID.toString('hex') + "\n" +
            "Prev: " + this.previd.toString('hex') + "\n" +
            "Version: " + this.version + "\n" +
            "Rules: " + this.rules;
    }

    public static newDarc(owners: Identity[], signers: Identity[], desc: Buffer): Darc {
        const darc = new Darc({
            version: Long.fromNumber(0, true),
            description: desc,
            baseID: Buffer.from([]),
            previd: createHash('sha256').digest(),
            rules: initRules(owners, signers),
        });

        return darc;
    }

    public static fromProof(p: Proof): Darc {
        if (!p.matchContract(DarcInstance.contractID)) {
            throw new Error(`mismatch contract ID: ${DarcInstance.contractID} != ${p.contractID.toString()}`);
        }

        return Darc.decode(p.value);
    }
}

registerMessage('Rule', Rule);
registerMessage('Rules', Rules);
registerMessage('Darc', Darc);
