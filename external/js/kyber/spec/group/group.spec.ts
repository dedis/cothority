import {
    availableCurves,
    edwards25519,
    newCurve,
    nist,
} from "../../src/curve";

describe("curves", () => {
    it("are all listed", () => {
        const allCurves = availableCurves();

        expect(allCurves).toContain("edwards25519");
        expect(allCurves).toContain("p256");
    });

    it("should not instantiate unknown curves", () => {
        expect(() => newCurve("unknown")).toThrowError("curve not known");
    });

    it("should instantiate ed25519 curves", () => {
        const ed25519 = newCurve("edwards25519");

        expect(ed25519).toEqual(jasmine.any(edwards25519.Curve));
        expect(ed25519.point().pick()).toEqual(jasmine.any(edwards25519.Point));
    });

    it("should instantiate p256 curves", () => {
        const p256 = newCurve("p256");

        expect(p256).toEqual(jasmine.any(nist.Curve));
        expect(p256.point().pick()).toEqual(jasmine.any(nist.Point));
    });
});
