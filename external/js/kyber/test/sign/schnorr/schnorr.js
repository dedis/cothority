const {
  sign: { schnorr },
  curve: { nist }
} = require("../../../dist/index.js");

var group = new nist.Curve(nist.Params.p256);
const secretKey = group.scalar().pick();
const publicKey = group.point().mul(secretKey, null);
const scalarLen = group.scalarLen();
const pointLen = group.pointLen();
const message = Buffer.from([1, 2, 3, 4]);

describe("schnorr signature", () => {
  it("returns a signature with good size", () => {
    var sig = schnorr.sign(group, secretKey, message);
    var sigSize = scalarLen + pointLen;

    expect(sig).toEqual(jasmine.any(Buffer));
    expect(sig.length).toBe(sigSize);
  });
});

describe("schnorr verification", () => {
  let sig;

  beforeAll(() => {
    sig = schnorr.sign(group, secretKey, message);
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
    const wrongSig = new Uint8Array(r.length + s.length);
    wrongSig.set(r);
    wrongSig.set(s, r.length);

    expect(schnorr.verify(group, publicKey, message, wrongSig)).toBeFalsy();
  });

  it("returns false for a wrong public key", () => {
    const wrongPub = group.point().pick();

    expect(schnorr.verify(group, wrongPub, message, sig)).toBeFalsy();
  });

  it("returns false for a wrong message", () => {
    const wrongMessage = new Uint8Array(message).reverse();

    expect(schnorr.verify(group, publicKey, wrongMessage, sig)).toBeFalsy();
  });

  it("returns true for a well formed signature", () => {
    expect(schnorr.verify(group, publicKey, message, sig)).toBeTruthy();
  });
});
