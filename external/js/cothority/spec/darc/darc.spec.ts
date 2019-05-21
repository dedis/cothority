import { Darc, IdentityEd25519, Rule, Rules } from "../../src/darc";
import IdentityDarc from "../../src/darc/identity-darc";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { SIGNER } from "../support/conondes";

describe("Darc Tests", () => {
    it("should create and evolve darcs", async () => {
        const darc = new Darc();
        const darc2 = darc.evolve();
        darc2.addIdentity("abc", new IdentityEd25519({point: SIGNER.point}), "");
        const darc3 = darc2.evolve();

        expect(darc3.version.toNumber()).toBe(2);
        expect(darc3.prevID).toEqual(darc2.id);
        expect(darc3.id).not.toEqual(darc2.id);
        expect(darc3.getBaseID()).toEqual(darc.getBaseID());
        expect(darc3.toString()).toBeDefined();
    });

    it("should correctly check rule matches", async () => {
        const sig1 = SignerEd25519.random();
        const sig2 = SignerEd25519.random();
        const sig3 = SignerEd25519.random();
        const d3 = Darc.createBasic([sig1], [sig3, sig2]);
        const d2 = Darc.createBasic([sig1], [sig3, new IdentityDarc({id: d3.getBaseID()})]);
        const d1 = Darc.createBasic([sig1], [sig3, new IdentityDarc({id: d2.getBaseID()})]);
        const darcs = [d1, d2, d3];
        const getDarc = (id: Buffer) => {
            for (const d of darcs) {
                if (d.getBaseID().equals(id)) {
                    return Promise.resolve(d);
                }
            }
            return Promise.reject();
        };
        expect((await d1.ruleMatch(Darc.ruleSign, [sig2], getDarc)).length).toBe(1);
        expect((await d1.ruleMatch(Darc.ruleSign, [sig3], getDarc)).length).toBe(1);
        expect((await d1.ruleMatch(Darc.ruleSign, [sig1], getDarc)).length).toBe(0);
    });

    it ("should correctly treat expressions", () => {
        let rule = new Rule({action: "sign"});
        const id1 = "identity1";
        const id2 = "identity2";
        const id3 = "identity3";
        expect(rule.append(id1, Rule.OR).toString()).toEqual(id1);
        rule = new Rule({action: "sign", expr: Buffer.from(id1)});
        expect(rule.expr.toString()).toEqual(id1);
        expect(rule.append(id2, Rule.OR).toString()).toEqual(`${id1} | ${id2}`);
        expect(rule.append(id3, Rule.OR).toString()).toEqual(`${id1} | ${id2} | ${id3}`);

        const ruleC = rule.clone();
        expect(() => ruleC.remove("id")).toThrowError();
        expect(() => ruleC.remove("darc")).toThrowError();
        expect(ruleC.remove(id2).toString()).toEqual(`${id1} | ${id3}`);
        expect(ruleC.remove(id3).toString()).toEqual(`${id1}`);
        expect(ruleC.remove(id1).toString()).toEqual("");

        expect(rule.remove(id1).toString()).toEqual(`${id2} | ${id3}`);

        const rules = new Rules({list: [rule]});
        expect(rules.getRule("sign").expr.toString().split("|").length).toBe(2);
        expect(rules.getRule("sign").remove(id2).toString()).toBe(id3);
        expect(rules.getRule("sign").expr.toString()).toBe(id3);
    });
});
