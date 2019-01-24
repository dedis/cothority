import Nist from '../../../src/curve/nist';

const { Curve, Params, Scalar, Point } = Nist;

describe('Nist curve', () => {
    const curve = new Curve(Params.p256);

    it("should return the name of the curve", () => {
        expect(curve.string()).toBe("P256");
    });

    it("scalarLen should return the length of scalar", () => {
        expect(curve.scalarLen()).toBe(32);
    });

    it("pointLen should return the length of point", () => {
        expect(curve.pointLen()).toBe(65);
    });

    it("scalar should return a scalar", () => {
        expect(curve.scalar()).toEqual(jasmine.any(Scalar));
    });

    it("point should return a point", () => {
        expect(curve.point()).toEqual(jasmine.any(Point));
    });
});
