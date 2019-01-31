"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const blake2xs_1 = require("@stablelib/blake2xs");
const __1 = require("../..");
const lodash_1 = require("lodash");
exports.Suite = __1.curve.newCurve('edwards25519');
class RingSig {
    constructor(c0, s, tag) {
        this.c0 = c0;
        this.s = s;
        this.tag = tag;
    }
    encode() {
        let bufs = [];
        bufs.push(this.c0.marshalBinary());
        for (let scalar of this.s) {
            bufs.push(scalar.marshalBinary());
        }
        if (this.tag) {
            bufs.push(this.tag.marshalBinary());
        }
        return Buffer.concat(bufs);
    }
    static fromBytes(signatureBuffer, isLinkableSig) {
        let scalarMarshalSize = exports.Suite.scalarLen();
        let pointMarshalSize = exports.Suite.pointLen();
        const c0 = exports.Suite.scalar();
        c0.unmarshalBinary(signatureBuffer.slice(0, pointMarshalSize));
        let S = [];
        let endIndex = isLinkableSig ? signatureBuffer.length - pointMarshalSize : signatureBuffer.length;
        for (let i = pointMarshalSize; i < endIndex; i += scalarMarshalSize) {
            const t = exports.Suite.scalar();
            t.unmarshalBinary(signatureBuffer.slice(i, i + scalarMarshalSize));
            S.push(t);
        }
        if (isLinkableSig) {
            const tag = exports.Suite.point();
            tag.unmarshalBinary(signatureBuffer.slice(endIndex));
            return new RingSig(c0, S, tag);
        }
        return new RingSig(c0, S);
    }
}
exports.RingSig = RingSig;
function sign(message, anonymitySet, secret, linkScope) {
    let hasLS = linkScope && (linkScope !== null);
    let pi = findSecretIndex(anonymitySet, secret);
    let n = anonymitySet.length;
    let L = anonymitySet.slice(0);
    let linkBase;
    let linkTag;
    if (hasLS) {
        let linkStream = new blake2xs_1.BLAKE2Xs(undefined, { key: linkScope });
        linkBase = exports.Suite.point().pick(createStreamFromBlake(linkStream));
        linkTag = exports.Suite.point().mul(secret, linkBase);
    }
    let H1pre = signH1pre(linkScope, linkTag, message);
    let u = exports.Suite.scalar().pick();
    let UB = exports.Suite.point().mul(u);
    let UL;
    if (hasLS) {
        UL = exports.Suite.point().mul(u, linkBase);
    }
    let s = [];
    let c = [];
    c[(pi + 1) % n] = signH1(H1pre, UB, UL);
    let P = exports.Suite.point();
    let PG = exports.Suite.point();
    let PH;
    if (hasLS) {
        PH = exports.Suite.point();
    }
    for (let i = (pi + 1) % n; i !== pi; i = (i + 1) % n) {
        s[i] = exports.Suite.scalar().pick();
        PG.add(PG.mul(s[i]), P.mul(c[i], L[i]));
        if (hasLS) {
            PH.add(PH.mul(s[i], linkBase), P.mul(c[i], linkTag));
        }
        c[(i + 1) % n] = signH1(H1pre, PG, PH);
    }
    s[pi] = exports.Suite.scalar();
    s[pi].mul(secret, c[pi]).sub(u, s[pi]);
    return new RingSig(c[0], s, linkTag);
}
exports.sign = sign;
function verify(message, anonymitySet, signatureBuffer, linkScope) {
    let n = anonymitySet.length;
    let L = anonymitySet.slice(0);
    let linkBase, linkTag;
    let sig = RingSig.fromBytes(signatureBuffer, !!linkScope);
    if (anonymitySet.length != sig.s.length) {
        throw new Error("given anonymity set and signature anonymity set not of equal length");
    }
    if (linkScope) {
        let linkStream = new blake2xs_1.BLAKE2Xs(undefined, { key: linkScope });
        linkBase = exports.Suite.point().pick(createStreamFromBlake(linkStream));
        linkTag = sig.tag;
    }
    let H1pre = signH1pre(linkScope, linkTag, message);
    let P, PG, PH;
    P = exports.Suite.point();
    PG = exports.Suite.point();
    if (linkScope) {
        PH = exports.Suite.point();
    }
    let s = sig.s;
    let ci = sig.c0;
    for (let i = 0; i < n; i++) {
        PG.add(PG.mul(s[i]), P.mul(ci, L[i]));
        if (linkScope) {
            PH.add(PH.mul(s[i], linkBase), P.mul(ci, linkTag));
        }
        ci = signH1(H1pre, PG, PH);
    }
    if (!ci.equal(sig.c0)) {
        return false;
    }
    return true;
}
exports.verify = verify;
function findSecretIndex(keys, privateKey) {
    const pubKey = exports.Suite.point().base().mul(privateKey);
    const pi = keys.findIndex(pub => pub.equal(pubKey));
    if (pi < 0) {
        throw new Error("didn't find public key in anonymity set");
    }
    return pi;
}
function createStreamFromBlake(blakeInstance) {
    function getNextBytes(count) {
        let array = new Uint8Array(count);
        blakeInstance.stream(array);
        return Buffer.from(array);
    }
    return getNextBytes;
}
function signH1pre(linkScope, linkTag, message) {
    const H1pre = new blake2xs_1.BLAKE2Xs(undefined, { key: message });
    if (linkScope) {
        H1pre.update(linkScope);
        const tag = linkTag.marshalBinary();
        H1pre.update(tag);
    }
    return H1pre;
}
function signH1(H1pre, PG, PH) {
    const H1 = lodash_1.cloneDeep(H1pre);
    const PGb = PG.marshalBinary();
    H1.update(PGb);
    if (PH) {
        const PHb = PH.marshalBinary();
        H1.update(PHb);
    }
    return exports.Suite.scalar().pick(createStreamFromBlake(H1));
}
