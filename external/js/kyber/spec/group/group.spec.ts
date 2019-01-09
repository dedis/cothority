import { availableCurves, newCurve, edwards25519 } from '../../src/curve';

describe("curves", () => {
    it("are all listed", () => {
        const allCurves = availableCurves();

        expect(allCurves).toContain("edwards25519");
        expect(allCurves).toContain("p256");
    });

    it("can be created by name", () => {
        const ed25519 = newCurve("edwards25519");

        expect(ed25519).toEqual(jasmine.any(edwards25519.Curve));
        expect(ed25519.point().pick()).toEqual(jasmine.any(edwards25519.Point));
        expect(() => newCurve("unknown")).toThrowError("curve not known");
    });
});
