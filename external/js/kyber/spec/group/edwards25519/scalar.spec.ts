import BN = require('bn.js');
import Curve from '../../../src/curve/edwards25519/curve';
import Scalar from '../../../src/curve/edwards25519/scalar';
import { PRNG } from '../../helpers/utils';

describe("Ed25519 Scalar", () => {
    const prng = new PRNG(42);
    const curve = new Curve();
    // prettier-ignore
    const b1 = Buffer.from([101, 216, 110, 23, 127, 7, 203, 250, 206, 170, 55, 91, 97, 239, 222, 159, 41, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 171]);
    // prettier-ignore
    const b2 = Buffer.from([88, 146, 91, 18, 158, 90, 102, 25, 82, 85, 219, 232, 60, 253, 138, 65, 183, 2, 157, 218, 70, 58, 193, 179, 212, 232, 104, 98, 125, 202, 176, 9]);

    it("should set the scalar reading bytes from little endian array", () => {
        const bytes = Buffer.from([2, 4, 8, 10]);
        const s = curve.scalar().setBytes(bytes) as Scalar;
        const target = new BN("0a080402", 16);

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
    });

    it("should reduce to number to mod Q", () => {
        // prettier-ignore
        const bytes = Buffer.from([255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255]);
        const s = curve.scalar().setBytes(bytes) as Scalar;
        const target = new BN(
            "0ffffffffffffffffffffffffffffffec6ef5bf4737dcf70d6ec31748d98951c",
            16
        );

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
    });

    it("should return true for equal scalars and false otherwise", () => {
        const bytes = Buffer.from([241, 51, 4]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = curve.scalar().setBytes(bytes);

        expect(s1.equals(s2)).toBeTruthy("s1 != s2");
        expect(s1.equals(curve.scalar())).toBeFalsy("s1 == 0");
    });

    it("should set the scalar to 0", () => {
        const s = curve.scalar().zero() as Scalar;
        const target = new BN(0, 16);

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
    });

    it("should make the receiver point to a scalar", () => {
        const bytes = Buffer.from([1, 2, 4, 52]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = curve.scalar().set(s1);
        const zero = curve.scalar().zero();

        expect(s1.equals(s2)).toBeTruthy("s1 != s2");
        s1.zero();
        expect(s1.equals(s2)).toBeTruthy("s1 != s2");
        expect(s1.equals(zero)).toBeTruthy("s1 != 0");
    });

    it("should clone a scalar", () => {
        const bytes = Buffer.from([1, 2, 4, 52]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = s1.clone();

        expect(s1.equals(s2)).toBeTruthy("s1 != s2");
        s1.zero();
        expect(s1.equals(s2)).toBeFalsy("s1 == s2");
    });

    it("should add two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const sum = curve.scalar().add(s1, s2);

        // prettier-ignore
        const target = Buffer.from([142, 79, 58, 43, 251, 31, 103, 75, 235, 66, 111, 67, 13, 48, 213, 251, 223, 252, 30, 150, 83, 181, 96, 87, 34, 5, 98, 17, 87, 61, 173, 5]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(sum.equals(s3)).toBeTruthy("sum != s3");
    });

    it("should subtract two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const diff = curve.scalar().sub(s1, s2);
        // prettier-ignore
        const target = Buffer.from([203, 254, 120, 99, 217, 205, 172, 112, 29, 53, 176, 20, 114, 47, 158, 141, 113, 247, 228, 224, 197, 64, 222, 239, 120, 51, 144, 76, 92, 168, 75, 2]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(diff.equals(s3)).toBeTruthy("diff != s3");
    });

    it("should negate a point", () => {
        const s1 = curve.scalar().setBytes(b1);
        const neg = curve.scalar().neg(s1);

        // prettier-ignore
        const target = Buffer.from([202, 66, 33, 231, 162, 58, 255, 205, 102, 18, 108, 165, 47, 205, 181, 69, 215, 5, 126, 68, 243, 132, 96, 92, 178, 227, 6, 81, 38, 141, 3, 4]);
        const s2 = curve.scalar();
        s2.unmarshalBinary(target);

        expect(neg.equals(s2)).toBeTruthy("neg != s2");
    });

    it("should set the scalar to one", () => {
        const one = curve.scalar().one();
        const bytes = one.marshalBinary();
        // prettier-ignore
        const target = Buffer.from([1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);

        expect(bytes).toEqual(target);
    });

    it("should multiply two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const prod = curve.scalar().mul(s1, s2);

        // prettier-ignore
        const target = Buffer.from([34, 211, 107, 76, 100, 86, 13, 215, 123, 147, 172, 207, 230, 235, 139, 24, 48, 176, 64, 192, 65, 15, 67, 221, 226, 55, 42, 236, 84, 151, 8, 7]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(prod.equals(s3)).toBeTruthy("mul != s3");
    });

    it("should divide two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const quotient = curve.scalar().div(s1, s2);

        // prettier-ignore
        const target = Buffer.from([145, 191, 30, 22, 157, 168, 12, 162, 220, 120, 243, 189, 108, 219, 155, 180, 153, 9, 224, 106, 128, 43, 50, 228, 38, 190, 218, 139, 185, 250, 4, 4]);
        const s3 = curve.scalar();
        s3.unmarshalBinary(target);

        expect(quotient.equals(s3)).toBeTruthy("quotient != s3");
    });

    it("should compute the inverse modulo n of scalar", () => {
        const s1 = curve.scalar().setBytes(b1);
        const inv = curve.scalar().inv(s1);

        // prettier-ignore
        const target = Buffer.from([154, 16, 208, 201, 223, 62, 219, 72, 103, 81, 202, 115, 69, 207, 192, 15, 46, 182, 202, 37, 102, 233, 116, 118, 239, 127, 234, 84, 12, 32, 206, 5]);
        const s2 = curve.scalar();
        s2.unmarshalBinary(target);

        expect(inv.equals(s2)).toBeTruthy();
    });

    it("should pick a random scalar", () => {
        prng.setSeed(42);
        const s1 = curve.scalar().pick(prng.pseudoRandomBytes);

        // prettier-ignore
        const bytes = Buffer.from([231, 30, 187, 110, 193, 139, 10, 170, 126, 79, 112, 41, 212, 167, 34, 46, 227, 253, 241, 189, 81, 181, 199, 179, 13, 151, 183, 143, 196, 244, 208, 1]);
        const target = curve.scalar();
        target.unmarshalBinary(bytes);

        expect(s1.equals(target)).toBeTruthy();
    });

    it("should return the marshalled representation of scalar", () => {
        const s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        const m = s1.marshalBinary();
        // prettier-ignore
        const target = Buffer.from([35, 145, 212, 117, 119, 40, 19, 138, 111, 138, 139, 253, 174, 44, 41, 207, 40, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 11]);

        expect(m).toEqual(target);
    });

    it("should convert marshalled representation to scalar", () => {
        const s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        // prettier-ignore
        const target = Buffer.from([35, 145, 212, 117, 119, 40, 19, 138, 111, 138, 139, 253, 174, 44, 41, 207, 40, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 11]);
        const bytes = s1.marshalBinary();

        expect(bytes).toEqual(target);
    });

    // not in case of Edwards25519
    xit("should throw an error if input > q", () => {
        const s1 = curve.scalar();

        expect(() => s1.unmarshalBinary(b1)).toThrow();
    });

    it("should throw an error if input size > marshalSize", () => {
        const s1 = curve.scalar() as Scalar;
        const data = Buffer.allocUnsafe(s1.marshalSize() + 1);

        expect(() => s1.unmarshalBinary(data)).toThrow();
    });

    it("should print the string representation of a scalar", () => {
        const s1 = curve.scalar();
        s1.unmarshalBinary(b1);
        // prettier-ignore
        const target = "2391d4757728138a6f8a8bfdae2c29cf28fa81bb0c7b9fa34d1cf9aed972fc0b";

        expect(s1.toString()).toBe(target);
    });

    // TODO: discrepency
    xit("should print the string representation of zero scalar", () => {
        let s1 = curve.scalar().zero() as Scalar;
        let target = "";
        expect(s1.toString()).toBe(target);
    });

    it("should print the string representation of one scalar", () => {
        const s1 = curve.scalar().one();
        const target =
            "0100000000000000000000000000000000000000000000000000000000000000";

        expect(s1.toString()).toBe(target);
    });
});
