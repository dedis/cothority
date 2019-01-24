"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const gfp12_1 = __importDefault(require("./gfp12"));
const bn_1 = require("./bn");
const twist_point_1 = __importDefault(require("./twist-point"));
const gfp2_1 = __importDefault(require("./gfp2"));
const gfp6_1 = __importDefault(require("./gfp6"));
const constants_1 = require("./constants");
const gfp_1 = __importDefault(require("./gfp"));
const sixuPlus2NAF = [0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 0, -1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, -1, 0, 1, 0, 0, 0, 1, 0, -1, 0, 0, 0, -1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, -1, 0, -1, 0, 0, 0, 0, 1, 0, 0, 0, 1];
/**
 * See the mixed addition algorithm from "Faster Computation of the
 * Tate Pairing", http://arxiv.org/pdf/0904.0854v3.pdf
 */
function lineFunctionAdd(r, p, q, r2) {
    const B = p.getX().mul(r.getT());
    const D = p.getY().add(r.getZ()).square().sub(r2).sub(r.getT()).mul(r.getT());
    const H = B.sub(r.getX());
    const I = H.square();
    let E = I.add(I);
    E = E.add(E);
    const J = H.mul(E);
    const L1 = D.sub(r.getY()).sub(r.getY());
    const V = r.getX().mul(E);
    let rx = L1.square().sub(J).sub(V).sub(V);
    let rz = r.getZ().add(H).square().sub(r.getT()).sub(I);
    let t = V.sub(rx).mul(L1);
    let t2 = r.getY().mul(J);
    t2 = t2.add(t2);
    let ry = t.sub(t2);
    let rt = rz.square();
    t = p.getY().add(rz).square().sub(r2).sub(rt);
    t2 = L1.mul(p.getX());
    t2 = t2.add(t2);
    const a = t2.sub(t);
    let c = rz.mulScalar(q.getY());
    c = c.add(c);
    let b = gfp2_1.default.zero().sub(L1).mulScalar(q.getX());
    b = b.add(b);
    return {
        a,
        b,
        c,
        rOut: new twist_point_1.default(rx, ry, rz, rt),
    };
}
/**
 * See the doubling algorithm for a=0 from "Faster Computation of the
 * Tate Pairing", http://arxiv.org/pdf/0904.0854v3.pdf
 */
function lineFunctionDouble(r, q) {
    const A = r.getX().square();
    const B = r.getY().square();
    const C = B.square();
    let D = r.getX().add(B).square().sub(A).sub(C);
    D = D.add(D);
    const E = A.add(A).add(A);
    const G = E.square();
    let rx = G.sub(D).sub(D);
    let rz = r.getY().add(r.getZ()).square().sub(B).sub(r.getT());
    let ry = D.sub(rx).mul(E);
    let t = C.add(C);
    t = t.add(t);
    t = t.add(t);
    ry = ry.sub(t);
    let rt = rz.square();
    t = E.mul(r.getT());
    t = t.add(t);
    const b = gfp2_1.default.zero().sub(t).mulScalar(q.getX());
    t = B.add(B);
    t = t.add(t);
    const a = r.getX().add(E).square().sub(A).sub(G).sub(t);
    let c = rz.mul(r.getT());
    c = c.add(c).mulScalar(q.getY());
    return {
        a,
        b,
        c,
        rOut: new twist_point_1.default(rx, ry, rz, rt),
    };
}
function mulLine(ret, res) {
    const a2 = new gfp6_1.default(gfp2_1.default.zero(), res.a, res.b).mul(ret.getX());
    const t3 = ret.getY().mulScalar(res.c);
    const t = res.b.add(res.c);
    const t2 = new gfp6_1.default(gfp2_1.default.zero(), res.a, t);
    let tx = ret.getX().add(ret.getY()).mul(t2).sub(a2).sub(t3);
    let ty = t3.add(a2.mulTau());
    return new gfp12_1.default(tx, ty);
}
/**
 * miller implements the Miller loop for calculating the Optimal Ate pairing.
 * See algorithm 1 from http://cryptojedi.org/papers/dclxvi-20100714.pdf
 */
function miller(q, p) {
    let ret = gfp12_1.default.one();
    const aAffine = q.clone();
    aAffine.makeAffine();
    const bAffine = p.clone();
    bAffine.makeAffine();
    const minusA = new twist_point_1.default();
    minusA.neg(aAffine);
    let r = aAffine.clone();
    let r2 = aAffine.getY().square();
    for (let i = sixuPlus2NAF.length - 1; i > 0; i--) {
        let res = lineFunctionDouble(r, bAffine);
        if (i != sixuPlus2NAF.length - 1) {
            ret = ret.square();
        }
        ret = mulLine(ret, res);
        r = res.rOut;
        if (sixuPlus2NAF[i - 1] == 1) {
            res = lineFunctionAdd(r, aAffine, bAffine, r2);
        }
        else if (sixuPlus2NAF[i - 1] == -1) {
            res = lineFunctionAdd(r, minusA, bAffine, r2);
        }
        else {
            continue;
        }
        ret = mulLine(ret, res);
        r = res.rOut;
    }
    const q1 = new twist_point_1.default(aAffine.getX().conjugate().mul(constants_1.xiToPMinus1Over3), aAffine.getY().conjugate().mul(constants_1.xiToPMinus1Over2), gfp2_1.default.one(), gfp2_1.default.one());
    const minusQ2 = new twist_point_1.default(aAffine.getX().mulScalar(new gfp_1.default(constants_1.xiToPSquaredMinus1Over3)), aAffine.getY(), gfp2_1.default.one(), gfp2_1.default.one());
    r2 = q1.getY().square();
    const res = lineFunctionAdd(r, q1, bAffine, r2);
    ret = mulLine(ret, res);
    r = res.rOut;
    r2 = minusQ2.getY().square();
    const res2 = lineFunctionAdd(r, minusQ2, bAffine, r2);
    return mulLine(ret, res2);
}
/**
 * finalExponentiation computes the (p¹²-1)/Order-th power of an element of
 * GF(p¹²) to obtain an element of GT (steps 13-15 of algorithm 1 from
 * http://cryptojedi.org/papers/dclxvi-20100714.pdf)
 */
function finalExponentiation(a) {
    let t1 = a.conjugate();
    t1 = t1.mul(a.invert());
    const t2 = t1.frobeniusP2();
    t1 = t1.mul(t2);
    const fp = t1.frobenius();
    const fp2 = t1.frobeniusP2();
    const fp3 = fp2.frobenius();
    const fu = t1.exp(constants_1.u);
    const fu2 = fu.exp(constants_1.u);
    const fu3 = fu2.exp(constants_1.u);
    const fu2p = fu2.frobenius();
    const fu3p = fu3.frobenius();
    const y0 = fp.mul(fp2).mul(fp3);
    const y1 = t1.conjugate();
    const y2 = fu2.frobeniusP2();
    const y3 = fu.frobenius().conjugate();
    const y4 = fu.mul(fu2p).conjugate();
    const y5 = fu2.conjugate();
    const y6 = fu3.mul(fu3p).conjugate();
    let t0 = y6.square().mul(y4).mul(y5);
    t1 = y3.mul(y5).mul(t0).square();
    t0 = t0.mul(y2);
    t1 = t1.mul(t0).square();
    t0 = t1.mul(y1);
    t1 = t1.mul(y0);
    return t0.square().mul(t1);
}
/**
 * Compute the pairing between a point in G1 and a point in G2
 * using the Optimal Ate algorithm
 * @param g1 the point in G1
 * @param g2 the point in G2
 * @returns the resulting point in GT
 */
function optimalAte(g1, g2) {
    const e = miller(g2.getPoint(), g1.getPoint());
    const ret = finalExponentiation(e);
    if (g1.isInfinity() || g2.isInfinity()) {
        return new bn_1.GT(gfp12_1.default.one());
    }
    return new bn_1.GT(ret);
}
exports.optimalAte = optimalAte;
