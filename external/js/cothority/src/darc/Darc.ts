import {Proof} from "~/lib/cothority/byzcoin/Proof";

const crypto = require("crypto-browserify");

import {Log} from "~/lib/Log";
import {objToProto, Root} from "~/lib/cothority/protobuf/Root";
import {Identity} from "~/lib/cothority/darc/Identity";
import {DarcInstance} from "~/lib/cothority/byzcoin/contracts/DarcInstance";

export class Rule {
    action: string;
    expr: Buffer;

    constructor(a: string, e: Buffer) {
        this.action = a;
        this.expr = e;
    }

    toString(): string {
        return this.action + " - " + this.expr.toString();
    }

    static fromIdentities(r: string, ids: Identity[], operator: string): Rule {
        let e = ids.map(id => {
            return id.toString();
        }).join(" " + operator + " ");
        return new Rule(r, new Buffer(e));
    }
}

export class Rules {
    list: Rule[];

    constructor() {
        this.list = [];
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

export class Darc {
    version: number;
    description: Buffer;
    baseid: Buffer;
    previd: Buffer;
    rules: Rules;

    constructor(buf: any) {
        this.version = buf.version;
        this.description = buf.description;
        this.baseid = buf.baseid;
        this.previd = buf.previd;
        this.rules = new Rules();
        buf.rules.list.forEach(r => {
            this.rules.list.push(new Rule(r.action, r.expr));
        })
    }

    getId(): Buffer {
        let h = crypto.createHash("sha256");
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

    toString(): string {
        return "ID: " + this.getId().toString('hex') + "\n" +
            "Base: " + this.getBaseId().toString('hex') + "\n" +
            "Prev: " + this.previd.toString('hex') + "\n" +
            "Version: " + this.version + "\n" +
            "Rules: " + this.rules;
    }

    toProto(): Buffer {
        return objToProto(this, "Darc");
    }

    static fromProto(buf: Buffer): Darc {
        const requestModel = Root.lookup("Darc");
        return new Darc(requestModel.decode(buf));
    }

    static fromRulesDesc(r: Rules, desc: string) {
        return new Darc({
            version: 0,
            description: new Buffer(desc),
            rules: r,
            baseid: new Buffer(0),
            previd: crypto.createHash("sha256").digest(),
        });
    }
}