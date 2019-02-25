import { Darc, IdentityEd25519 } from "../../src/darc";
import { SIGNER } from "../support/conondes";

describe("Darc Tests", () => {
    it("should create and evolve darcs", async () => {
        const darc = new Darc();
        const darc2 = darc.evolve();
        darc2.addIdentity("abc", new IdentityEd25519({ point: SIGNER.point }), "");
        const darc3 = darc2.evolve();

        expect(darc3.version.toNumber()).toBe(2);
        expect(darc3.prevID).toEqual(darc2.id);
        expect(darc3.id).not.toEqual(darc2.id);
        expect(darc3.getGenesisDarcID()).toEqual(darc.getGenesisDarcID());
        expect(darc3.toString()).toBeDefined();
    });
});
