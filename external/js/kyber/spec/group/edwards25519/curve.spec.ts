import BN from "bn.js";
import Curve from '../../../src/curve/edwards25519/curve';
import Scalar from '../../../src/curve/edwards25519/scalar';
import Point from '../../../src/curve/edwards25519/point';

describe("edwards25519", () => {
    const curve = new Curve();
  
    it("should return the name of the curve", () => {
      expect(curve.string()).toBe("Ed25519", "Curve name is not correct");
    });
  
    it("scalarLen should return the length of scalar", () => {
      expect(curve.scalarLen()).toBe(32, "Scalar length not correct");
    });
  
    it("pointLen should return the length of point", () => {
      expect(curve.pointLen()).toBe(32, "Point length not correct");
    });
  
    it("scalar should return a scalar", () => {
      expect(curve.scalar()).toEqual(jasmine.any(Scalar));
    });
  
    it("point should return a point", () => {
      expect(curve.point()).toEqual(jasmine.any(Point));
    });

    it('should generate a private key multiple of 8', () => {
      const key = curve.newKey();
      const eight = curve.scalar().setBytes(new BN(8).toBuffer());
      const quotient = curve.scalar().div(key, eight);

      expect(curve.scalar().mul(eight, quotient).equals(key)).toBeTruthy();
    });
});
