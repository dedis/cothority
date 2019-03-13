import { createHash } from "crypto";
import Long from "long";
import { Message, Properties } from "protobufjs/light";
import DarcInstance from "../byzcoin/contracts/darc-instance";
import Proof from "../byzcoin/proof";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import { IIdentity } from "./identity-wrapper";
import Rules from "./rules";

/**
 * Create a list of rules with basic permissions for owners and signers
 * @param owners those allow to evolve the darc
 * @param signers those allow to sign
 * @returns the list of rules
 */
function initRules(owners: IIdentity[], signers: IIdentity[]): Rules {
    const rules = new Rules();

    owners.forEach((o) => rules.appendToRule("invoke:darc.evolve", o, Rules.AND));
    signers.forEach((s) => rules.appendToRule("_sign", s, Rules.OR));

    return rules;
}

/**
 * Distributed Access Right Controls
 */
export default class Darc extends Message<Darc> {
    static register() {
        registerMessage("Darc", Darc, Rules);
    }

    /**
     * Create a genesis darc using the owners and signers to populate the
     * rules
     * @param owners    those you can evolve the darc
     * @param signers   those you can sign
     * @param desc      the description of the darc
     * @returns the new darc
     */
    static newDarc(owners: IIdentity[], signers: IIdentity[], desc?: Buffer): Darc {
        const darc = new Darc({
            baseID: Buffer.from([]),
            description: desc,
            prevID: createHash("sha256").digest(),
            rules: initRules(owners, signers),
            version: Long.fromNumber(0, true),
        });

        return darc;
    }

    /**
     * Instantiate a darc using a proof
     * @param key   Key of the proof
     * @param p     The proof to use
     * @returns the darc when compatible
     */
    static fromProof(key: Buffer, p: Proof): Darc {
        if (!p.matchContract(DarcInstance.contractID)) {
            throw new Error(`mismatch contract ID: ${DarcInstance.contractID} != ${p.contractID}`);
        }

        if (!p.exists(key)) {
            throw new Error(`invalid key for proof: ${key.toString("hex")}`);
        }

        return Darc.decode(p.value);
    }

    readonly version: Long;
    readonly description: Buffer;
    readonly baseID: Buffer;
    readonly prevID: Buffer;
    readonly rules: Rules;

    constructor(properties?: Properties<Darc>) {
        super(properties);

        this.description = Buffer.from(this.description || EMPTY_BUFFER);
        this.baseID = Buffer.from(this.baseID || EMPTY_BUFFER);
        this.prevID = Buffer.from(this.prevID || EMPTY_BUFFER);
        this.rules = this.rules || new Rules();

        /* Protobuf aliases */

        Object.defineProperty(this, "baseid", {
            get(): Buffer {
                return this.baseID;
            },
            set(value: Buffer) {
                this.baseID = value;
            },
        });

        Object.defineProperty(this, "previd", {
            get(): Buffer {
                return this.prevID;
            },
            set(value: Buffer) {
                this.prevID = value;
            },
        });
    }

    /**
     * Get the id of the darc
     * @returns the id as a buffer
     */
    get id(): Buffer {
        const h = createHash("sha256");
        const versionBuf = Buffer.from(this.version.toBytesLE());
        h.update(versionBuf);
        h.update(this.description);

        if (this.baseID.length > 0) {
            h.update(this.baseID);
        }
        if (this.prevID.length > 0) {
            h.update(this.prevID);
        }

        this.rules.list.forEach((r) => {
            h.update(r.action);
            h.update(r.expr);
        });

        return h.digest();
    }

    /**
     * Get the id of the genesis darc
     * @returns the id as a buffer
     */
    getGenesisDarcID(): Buffer {
        if (this.version.eq(0)) {
            return this.id;
        } else {
            return this.baseID;
        }
    }

    /**
     * Append an identity to a rule using the given operator when
     * it already exists
     * @param rule      the name of the rule
     * @param identity  the identity to append to the rule
     * @param op        the operator to use if necessary
     */
    addIdentity(rule: string, identity: IIdentity, op: string): void {
        this.rules.appendToRule(rule, identity, op);
    }

    /**
     * Copy and evolve the darc to the next version so that it can be
     * changed and proposed to byzcoin.
     * @returns a new darc
     */
    evolve(): Darc {
        return new Darc({
            baseID: this.getGenesisDarcID(),
            description: this.description,
            prevID: this.id,
            rules: this.rules.clone(),
            version: this.version.add(1),
        });
    }

    /**
     * Get a string representation of the darc
     * @returns the string representation
     */
    toString(): string {
        return "ID: " + this.id.toString("hex") + "\n" +
            "Base: " + this.baseID.toString("hex") + "\n" +
            "Prev: " + this.prevID.toString("hex") + "\n" +
            "Version: " + this.version + "\n" +
            "Rules: " + this.rules;
    }

    /**
     * Helper to encode the darc using protobuf
     * @returns encoded darc as a buffer
     */
    toBytes(): Buffer {
        return Buffer.from(Darc.encode(this).finish());
    }
}

Darc.register();
