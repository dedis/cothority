import { createHash } from 'crypto';
import {Identity} from "../darc/Identity";
import { Message, Properties } from "protobufjs";

export class Rule extends Message<Rule> {
    action: string;
    expr: Buffer;

    toString(): string {
        return this.action + " - " + this.expr.toString();
    }

    static fromIdentities(r: string, ids: Identity[], operator: string): Rule {
        const e = ids.map(id => id.toString()).join(" " + operator + " ");

        return new Rule({ action: r, expr: new Buffer(e) });
    }
}

export class Rules extends Message<Rules> {
    public static OR = '|';

    readonly list: Rule[];

    constructor(properties?: Properties<Rules>) {
        super(properties);

        if (!this.list) {
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

    static fromOwnersSigners(os: Identity[], ss: Identity[]): Rules {
        let r = new Rules();
        r.list.push(Rule.fromIdentities("invoke:evolve", os, "&"));
        r.list.push(Rule.fromIdentities("_sign", ss, "|"));
        return r;
    }
}

export class Darc extends Message<Darc> {
    readonly version: number;
    readonly description: Buffer;
    readonly baseid: Buffer;
    readonly previd: Buffer;
    readonly rules: Rules;

    getId(): Buffer {
        let h = createHash("sha256");
        let versionBuf = new Buffer(8);
        versionBuf.writeUInt32LE(this.version, 0);
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

    getBaseId(): Buffer {
        if (this.version == 0) {
            return this.getId();
        } else {
            return this.baseid;
        }
    }

    addIdentity(rule: string, identity: Identity, op: string): void {
        this.rules.appendToRule(rule, identity, op);
    }

    toString(): string {
        return "ID: " + this.getId().toString('hex') + "\n" +
            "Base: " + this.getBaseId().toString('hex') + "\n" +
            "Prev: " + this.previd.toString('hex') + "\n" +
            "Version: " + this.version + "\n" +
            "Rules: " + this.rules;
    }

    public static newDarc(owners: Identity[], signers: Identity[], desc: Buffer): Darc {
        const darc = new Darc({
            version: 0,
            description: desc,
            baseid: new Buffer(0),
            previd: createHash('sha256').digest(),
            rules: new Rules(),
        });

        return darc;
    }
}