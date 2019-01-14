import Nist from '../../../src/curve/nist';
import { schnorr } from '../../../src/sign';

const { Curve, Params } = Nist;
const { sign, verify } = schnorr;

describe("Schnorr Signature", () => {
    const group = new Curve(Params.p256);
    const secretKey = group.scalar().pick();
    const publicKey = group.point().mul(secretKey, null);
    const scalarLen = group.scalarLen();
    const pointLen = group.pointLen();
    const message = Buffer.from([1, 2, 3, 4]);

    it("returns a signature with good size", () => {
        var sig = sign(group, secretKey, message);
        var sigSize = scalarLen + pointLen;

        expect(sig).toEqual(jasmine.any(Buffer));
        expect(sig.length).toBe(sigSize);
    });

    it("returns false for a wrong signature", () => {
        const r = group
            .point()
            .pick()
            .marshalBinary();
        const s = group
            .point()
            .pick()
            .marshalBinary();
        const wrongSig = Buffer.alloc(r.length + s.length);
        r.copy(wrongSig);
        s.copy(wrongSig, r.length);

        expect(verify(group, publicKey, message, wrongSig)).toBeFalsy();
    });

    it("returns false for a wrong public key", () => {
        const wrongPub = group.point().pick();
        const sig = sign(group, secretKey, message);

        expect(verify(group, wrongPub, message, sig)).toBeFalsy();
    });

    it("returns false for a wrong message", () => {
        const sig = sign(group, secretKey, message);
        const wrongMessage = Buffer.from([1, 2, 3]);

        expect(verify(group, publicKey, wrongMessage, sig)).toBeFalsy();
    });

    it("returns true for a well formed signature", () => {
        const sig = sign(group, secretKey, message);

        expect(verify(group, publicKey, message, sig)).toBeTruthy();
    });
});
