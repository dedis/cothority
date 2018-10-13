const chai = require("chai");
const expect = chai.expect;

const kyber = require("../../index.js");
const edwards25519 = kyber.curve.edwards25519;
const share = require("../../lib/share/index.js");
const BN = require("bn.js");

describe("poly", () => {  
    var group = new edwards25519.Curve();
    const secret = group.scalar().pick();
    const T = 3;
    const p = new share.PriPoly(group, T, secret);
    it("should hold T items", () => {
        expect(p.coeff.length).to.eq(T);
        expect(p.coeff[0]).to.eq(secret);
        expect(p.coeff[1]).to.not.be.undefined;
        expect(p.coeff[1]).to.not.eq(secret);
    });
    it("should make shares", () => {
        expect(p).is.not.undefined;
        const shares = p.shares(5);
        expect(shares).to.be.instanceOf(Array);
        expect(shares[0]).to.be.instanceOf(share.PriShare);
    });
    it("recover secret should work", () => {
        const shares = p.shares(5);
        // send the first 3 shares in
        const s2 = share.RecoverSecret(group, shares.slice(0,3), p.T, shares.length);
        expect(s2).to.eq(secret);
    });
});