import { Message, Properties } from "protobufjs/light";
import { InstanceID } from "../byzcoin";
import Log from "../log";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import { IIdentity } from "./identity-wrapper";

/**
 * A rule will give who is allowed to use a given action
 */
export class Rule extends Message<Rule> {

    static readonly OR = "|";
    static readonly AND = "&";

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Rule", Rule);
    }

    /**
     * Creates a rule given an action and an expression.
     *
     * @param action
     * @param expr
     */
    static fromActionExpr(action: string, expr: Buffer) {
        const r = new Rule({action});
        r.append(expr.toString(), null);
        return r;
    }

    readonly action: string;
    private expr: Buffer;

    constructor(props?: Properties<Rule>) {
        super(props);

        this.expr = Buffer.from(this.expr || EMPTY_BUFFER);
    }

    /**
     * Get a deep clone of the rule
     * @returns the new rule
     */
    clone(): Rule {
        return Rule.fromActionExpr(this.action, this.expr);
    }

    /**
     * Appends an identity given as a string to the expression and returns a copy of the
     * new expression.
     *
     * @param identity the identity to add, given as a string
     * @param op the operator to apply to the expression
     */
    append(identity: string, op: string): Buffer {
        if (this.expr.length > 0) {
            this.expr = Buffer.from(`${this.expr.toString()} ${op} ${identity}`);
        } else {
            this.expr = Buffer.from(identity);
        }
        return Buffer.from(this.expr);
    }

    /**
     * Searches for the given identity and removes it from the expression. Currently only
     * expressions containing Rule.OR are supported. It returns a copy of the new expression.
     *
     * @param identity the string representation of the identity
     */
    remove(identity: string): Buffer {
        let expr = this.expr.toString();
        if (expr.match(/(\(|\)|\&)/)) {
            throw new Error("don't know how to remove identity from expression with () or Rule.AND");
        }
        const matchReg = new RegExp(`\\b${identity}\\b`);
        if (!expr.match(matchReg)) {
            throw new Error("this identity is not part of the rule");
        }
        expr = expr.replace(matchReg, "");
        expr = expr.replace(/\|\s*\|/, "|");
        expr = expr.replace(/\s*\|\s*$/, "");
        expr = expr.replace(/^\s*\|\s*/, "");
        this.expr = Buffer.from(expr);
        return Buffer.from(this.expr);
    }

    /**
     * Returns the identities in the expression, in the case it is a single identity,
     * or if the identities are ORed together. If there are brakcets '()' or AND in the
     * expression, it will throw an error.
     */
    getIdentities(): string[] {
        if (this.expr.toString().match(/\(&/)) {
            throw new Error('Don\'t know what to do with "(" or "&" in expression');
        }
        return this.expr.toString().split("|").map((e) => e.trim());
    }

    /**
     * Returns a copy of the expression.
     */
    getExpr(): Buffer {
        return Buffer.from(this.expr);
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

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Rules", Rules, Rule);
    }

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
    appendToRule(action: string, identity: IIdentity, op: string): void {
        const idx = this.list.findIndex((r) => r.action === action);

        if (idx >= 0) {
            this.list[idx].append(identity.toString(), op);
        } else {
            this.list.push(Rule.fromActionExpr(action, Buffer.from(identity.toString())));
        }
    }

    /**
     * Sets a rule to correspond to the given identity. If the rule already exists, it will be
     * replaced.
     * @param action    the name of the rule
     * @param identity  the identity to append
     */
    setRule(action: string, identity: IIdentity): void {
        this.setRuleExp(action, Buffer.from(identity.toString()));
    }

    /**
     * Sets the expression of a rule. If the rule already exists, it will be replaced. If the
     * rule does not exist yet, it will be appended to the list of rules.
     * @param action the name of the rule
     * @param expression the expression to put in the rule
     */
    setRuleExp(action: string, expression: Buffer) {
        const idx = this.list.findIndex((r) => r.action === action);

        const nr = Rule.fromActionExpr(action, expression);
        if (idx >= 0) {
            this.list[idx] = nr;
        } else {
            this.list.push(nr);
        }
    }

    /**
     * Removes a given rule from the list.
     *
     * @param action the action that will be removed.
     */
    removeRule(action: string) {
        const pos = this.list.findIndex((rule) => rule.action === action);
        if (pos >= 0) {
            this.list.splice(pos);
        }
    }

    /**
     * getRule returns the rule with the given action
     *
     * @param action to search in the rules for.
     */
    getRule(action: string): Rule {
        return this.list.find((r) => r.action === action);
    }

    /**
     * Get a deep copy of the list of rules
     * @returns the clone
     */
    clone(): Rules {
        return new Rules({list: this.list.map((r) => r.clone())});
    }

    /**
     * Get a string representation of the rules
     * @returns a string representation
     */
    toString(): string {
        return this.list.map((l) => l.toString()).join("\n");
    }
}

Rule.register();
Rules.register();
