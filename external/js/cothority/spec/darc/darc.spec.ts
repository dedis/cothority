import { Darc, IdentityEd25519 } from "../../src/darc";
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
});
