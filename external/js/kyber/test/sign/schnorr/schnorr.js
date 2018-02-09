const mocha = require("mocha");
const chai = require("chai");
const assert = chai.assert;

const kyber = require("../../../index.js");
const schnorr = kyber.sign.schnorr;
// XXX to be changed to an interface style
const nist = kyber.curve.nist;
var group = new nist.Curve(nist.Params.p256);
const secretKey = group.scalar().pick();
const publicKey = group.point().mul(secretKey, null);
const scalarLen = group.scalarLen();
const pointLen = group.pointLen();
const message = new Uint8Array([1, 2, 3, 4]);

describe("schnorr signature", () => {
  it("fails with wrong suite input", () => {
    chai
      .expect(schnorr.sign.bind(null, new Number(2), null, null))
      .to.throw("first argument must be a suite");
  });

  it("fails with wrong private key input", () => {
    chai
      .expect(schnorr.sign.bind(null, group, new Number(3)))
      .to.throw("second argument must be a scalar");
  });

  it("fails with wrong message input", () => {
    chai
      .expect(schnorr.sign.bind(null, group, secretKey, true))
      .to.throw("third argument must be Uint8Array");
  });

  it("returns a uint8array signature with good size", () => {
    var sig = schnorr.sign(group, secretKey, message);
    var sigSize = scalarLen + pointLen;
    chai.expect(sig).to.be.an.instanceof(Uint8Array);
    assert.lengthOf(sig, sigSize);
  });
});

describe("schnorr verification", () => {
  var sig = schnorr.sign(group, secretKey, message);

  it("fails with a wrong suite input", () => {
    chai
      .expect(schnorr.verify.bind(null, new Number(2)))
      .to.throw("first argument must be a suite");
  });

  it("fails with a wrong public key input", () => {
    chai
      .expect(schnorr.verify.bind(null, group, new Number(2)))
      .to.throw("second argument must be a point");
  });

  it("fails with a wrong message input", () => {
    chai
      .expect(schnorr.verify.bind(null, group, publicKey, new Number(2)))
      .to.throw("third argument must be a Uint8Array");
  });

  it("fails with a wrong signature input", () => {
    chai
      .expect(
        schnorr.verify.bind(null, group, publicKey, message, new Number(2))
      )
      .to.throw("fourth argument must be a Uint8Array");
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
    assert.isFalse(schnorr.verify(group, publicKey, message, wrongSig));
  });

  it("returns false for a wrong public key", () => {
    const wrongPub = group.point().pick();
    assert.isFalse(schnorr.verify(group, wrongPub, message, sig));
  });

  it("returns false for a wrong message", () => {
    const wrongMessage = new Uint8Array(message).reverse();
    assert.isFalse(schnorr.verify(group, publicKey, wrongMessage, sig));
  });

  it("returns true for a well formed signature", () => {
    assert.isTrue(schnorr.verify(group, publicKey, message, sig));
  });
});
