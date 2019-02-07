import { Message, Properties } from "protobufjs";
import { registerMessage } from "../protobuf";
import Identity from "./identity";

/**
 * A rule will give who is allowed to use a given action
 */
export class Rule extends Message<Rule> {
    action: string;
    expr: Buffer;

    /**
     * Get a deep clone of the rule
     * @returns the new rule
     */
    clone(): Rule {
        return new Rule({
            action: this.action,
            expr: Buffer.from(this.expr),
        });
    }

    /**
     * Get a string representation of the rule
     * @returns the string representation
     */
    toString(): string {
        return this.action + " - " + this.expr.toString();
    }
}

/**
 * Wrapper around a list of rules that provides helpers to manage
 * the rules
 */
export default class Rules extends Message<Rules> {
    static OR = "|";
    static AND = "&";

    readonly list: Rule[];

    constructor(properties?: Properties<Rules>) {
        super(properties);

        if (!properties || !this.list) {
            this.list = [];
        }
    }

    /**
     * Create or update a rule with the given identity
     * @param action    the name of the rule
     * @param identity  the identity to append
     * @param op        the operator to use if the rule exists
     */
    appendToRule(action: string, identity: Identity, op: string): void {
        const rule = this.list.find((r) => r.action === action);

        if (rule) {
            rule.expr = Buffer.concat([rule.expr, Buffer.from(` ${op} ${identity.toString()}`)]);
        } else {
            this.list.push(new Rule({ action, expr: Buffer.from(identity.toString()) }));
        }
    }

    /**
     * Get a deep copy of the list of rules
     * @returns the clone
     */
    clone(): Rules {
        return new Rules({ list: this.list.map((r) => r.clone()) });
    }

    /**
     * Get a string representation of the rules
     * @returns a string representation
     */
    toString(): string {
        return this.list.map((l) => l.toString()).join("\n");
    }
}

registerMessage("Rule", Rule);
registerMessage("Rules", Rules);
