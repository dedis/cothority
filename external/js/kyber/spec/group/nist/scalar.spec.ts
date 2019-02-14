import BN = require('bn.js');
import Nist from '../../../src/curve/nist';
import NistScalar from '../../../src/curve/nist/scalar';
import { PRNG } from '../../helpers/utils';

const { Curve, Params } = Nist;

describe("Nist Scalar", () => {
    const prng = new PRNG(42);
    const curve = new Curve(Params.p256);
    // prettier-ignore
    const b1 = Buffer.from([101, 216, 110, 23, 127, 7, 203, 250, 206, 170, 55, 91, 97, 239, 222, 159, 41, 250, 129, 187, 12, 123, 159, 163, 77, 28, 249, 174, 217, 114, 252, 171]);
    // prettier-ignore
    const b2 = Buffer.from([88, 146, 91, 18, 158, 90, 102, 25, 82, 85, 219, 232, 60, 253, 138, 65, 183, 2, 157, 218, 70, 58, 193, 179, 212, 232, 104, 98, 125, 202, 176, 9]);

    beforeEach(() => prng.setSeed(42));

    it("should set the scalar reading bytes from big endian array", () => {
        const bytes = Buffer.from([2, 4, 8, 10]);
        const s = curve.scalar().setBytes(bytes) as NistScalar;
        const target = new BN("0204080a", 16);

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
    });

    it("should reduce to number to mod N", () => {
        // prettier-ignore
        const bytes = Buffer.from([255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255]);
        const s = curve.scalar().setBytes(bytes) as NistScalar;
        const target = new BN(
            "ffffffff00000000000000004319055258e8617b0c46353d039cdaae",
            16
        );

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
    });

    it("should return true for equal scalars and false otherwise", () => {
        const bytes = Buffer.from([241, 51, 4]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = curve.scalar().setBytes(bytes);

        expect(s1.equals(s2)).toBeTruthy();
        expect(s1.equals(curve.scalar())).toBeFalsy();
    });

    it("should set the scalar to 0", () => {
        const s = curve.scalar().zero() as NistScalar;
        const target = new BN(0, 16);

        expect(s.ref.arr.fromRed().cmp(target)).toBe(0);
    });

    it("should make the receiver point to a scalar", () => {
        const bytes = Buffer.from([1, 2, 4, 52]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = curve.scalar().set(s1);
        const zero = curve.scalar().zero();

        expect(s1.equals(s2)).toBeTruthy();
        s1.zero();
        expect(s1.equals(s2)).toBeTruthy();
        expect(s1.equals(zero)).toBeTruthy();
    });

    it("should clone a scalar", () => {
        const bytes = Buffer.from([1, 2, 4, 52]);
        const s1 = curve.scalar().setBytes(bytes);
        const s2 = s1.clone();

        expect(s1.equals(s2)).toBeTruthy();
        s1.zero();
        expect(s1.equals(s2)).toBeFalsy();
    });

    it("should add two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const sum = curve.scalar().add(s1, s2);

        // prettier-ignore
        const target = Buffer.from([190, 106, 201, 42, 29, 98, 50, 20, 33, 0, 19, 67, 158, 237, 104, 224, 224, 253, 31, 149, 82, 182, 97, 87, 34, 5, 98, 17, 87, 61, 172, 180]);
        const s3 = curve.scalar().setBytes(target);

        expect(sum.equals(s3)).toBeTruthy();
    });

    it("should subtract two scalars", () => {
        const b1 = Buffer.from([1, 2, 3, 4]);
        const b2 = Buffer.from([5, 6, 7, 8]);
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const diff = curve.scalar().sub(s1, s2);
        // prettier-ignore
        const target = Buffer.from([255, 255, 255, 255, 0, 0, 0, 0, 255, 255, 255, 255, 255, 255, 255, 255, 188, 230, 250, 173, 167, 23, 158, 132, 243, 185, 202, 194, 248, 95, 33, 77]);
        const s3 = curve.scalar().setBytes(target);

        expect(diff.equals(s3)).toBeTruthy();
    });

    it("should negate a point", () => {
        const s1 = curve.scalar().setBytes(b1);
        const neg = curve.scalar().neg(s1);

        // prettier-ignore
        const target = Buffer.from([154, 39, 145, 231, 128, 248, 52, 6, 49, 85, 200, 164, 158, 16, 33, 96, 146, 236, 120, 242, 154, 155, 254, 225, 166, 156, 209, 20, 34, 240, 40, 166]);
        const s2 = curve.scalar().setBytes(target);

        expect(neg.equals(s2)).toBeTruthy();
    });

    it("should return the bytes in big-endian representation", () => {
        const s1 = curve.scalar().setBytes(b1) as NistScalar;
        const bytes = s1.bytes();

        expect(b1).toEqual(bytes);
    });

    it("should set the scalar to one", () => {
        const one = curve.scalar().one() as NistScalar;
        const bytes = one.bytes();
        const target = Buffer.from([1]);
        
        expect(bytes).toEqual(target);
    });

    it("should multiply two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const prod = curve.scalar().mul(s1, s2);

        // prettier-ignore
        const target = Buffer.from([88, 150, 22, 208, 89, 155, 151, 255, 177, 162, 187, 27, 200, 24, 106, 226, 148, 44, 50, 249, 104, 23, 185, 233, 226, 79, 51, 233, 132, 194, 166, 138]);
        const s3 = curve.scalar().setBytes(target);

        expect(prod.equals(s3)).toBeTruthy();
    });

    it("should divide two scalars", () => {
        const s1 = curve.scalar().setBytes(b1);
        const s2 = curve.scalar().setBytes(b2);
        const quotient = curve.scalar().div(s1, s2);

        // prettier-ignore
        const target = Buffer.from([197, 214, 67, 20, 213, 10, 109, 3, 187, 62, 94, 90, 111, 152, 254, 126, 57, 162, 144, 250, 104, 92, 124, 206, 143, 31, 20, 64, 4, 243, 185, 241]);
        const s3 = curve.scalar().setBytes(target);

        expect(quotient.equals(s3)).toBeTruthy();
    });

    it("should compute the inverse modulo n of scalar", () => {
        const s1 = curve.scalar().setBytes(b1);
        const inv = curve.scalar().inv(s1);

        // prettier-ignore
        const target = Buffer.from([65, 112, 236, 29, 4, 150, 6, 224, 144, 13, 175, 197, 232, 73, 19, 137, 150, 235, 201, 127, 55, 45, 109, 196, 104, 154, 215, 9, 171, 186, 177, 58]);
        const s2 = curve.scalar().setBytes(target);

        expect(inv.equals(s2)).toBeTruthy();
    });

    it("should pick a random scalar", () => {
        const s1 = curve.scalar().pick(prng.pseudoRandomBytes);

        // prettier-ignore
        const bytes = Buffer.from([225, 208, 244, 196, 143, 183, 151, 13, 179, 199, 181, 81, 189, 241, 253, 227, 46, 34, 167, 212, 41, 112, 79, 126, 170, 10, 139, 193, 110, 187, 30, 231]);
        const target = curve.scalar().setBytes(bytes);

        expect(s1.equals(target)).toBeTruthy();
    });

    it("should return the marshalled representation of scalar", () => {
        const s1 = curve.scalar().setBytes(b1);
        const m = s1.marshalBinary();

        expect(m).toEqual(b1);
    });

    it("should convert marshalled representation to scalar", () => {
        const s1 = curve.scalar() as NistScalar;
        s1.unmarshalBinary(b1);
        const bytes = s1.bytes();

        expect(bytes).toEqual(b1);
    });

    it("should throw an error if input > q", () => {
        const s1 = curve.scalar();
        // prettier-ignore
        const bytes = Buffer.from([255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255]);
        
        expect(() => s1.unmarshalBinary(bytes)).toThrow();
    });

    it("should throw an error if input size > marshalSize", () => {
        const s1 = curve.scalar() as NistScalar;
        const data = Buffer.alloc(s1.marshalSize() + 1, 0);

        expect(() => s1.unmarshalBinary(data)).toThrow();
    });

    it("should print the string representation of a scalar", () => {
        const s1 = curve.scalar().setBytes(b1) as NistScalar;
        // prettier-ignore
        const target = "65d86e177f07cbfaceaa375b61efde9f29fa81bb0c7b9fa34d1cf9aed972fcab";

        expect(s1.toString()).toBe(target);
    });

    it("should print the string representation of zero scalar", () => {
        const s1 = curve.scalar().zero() as NistScalar;
        const target = "00";

        expect(s1.toString()).toBe(target);
    });

    it("should print the string representation of one scalar", () => {
        const s1 = curve.scalar().one() as NistScalar;
        const target = "01";

        expect(s1.toString()).toBe(target);
    });
});
