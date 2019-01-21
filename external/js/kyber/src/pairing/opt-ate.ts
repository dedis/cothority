import GfP12 from "./gfp12";
import { G1, G2, GT } from "./bn";
import TwistPoint from "./twist-point";
import CurvePoint from "./curve-point";
import GfP2 from "./gfp2";
import GfP6 from "./gfp6";
import { u, xiToPMinus1Over3, xiToPMinus1Over2, xiToPSquaredMinus1Over3 } from "./constants";
import GfP from "./gfp";

const sixuPlus2NAF = [0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 0, -1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, -1, 0, 1, 0, 0, 0, 1, 0, -1, 0, 0, 0, -1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, -1, 0, -1, 0, 0, 0, 0, 1, 0, 0, 0, 1];

/**
 * Results from the line functions
 */
interface Result {
    a: GfP2,
    b: GfP2,
    c: GfP2,
    rOut: TwistPoint,
}

/**
 * See the mixed addition algorithm from "Faster Computation of the
 * Tate Pairing", http://arxiv.org/pdf/0904.0854v3.pdf
 */
function lineFunctionAdd(r: TwistPoint, p: TwistPoint, q: CurvePoint, r2: GfP2): Result {
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
    let t2 = r.getY().mul(J)
    t2 = t2.add(t2);
    let ry = t.sub(t2);
    let rt = rz.square();

    t = p.getY().add(rz).square().sub(r2).sub(rt);
    t2 = L1.mul(p.getX());
    t2 = t2.add(t2);

    const a = t2.sub(t);
    let c = rz.mulScalar(q.getY());
    c = c.add(c);

    let b = GfP2.zero().sub(L1).mulScalar(q.getX());
    b = b.add(b);
    
    return {
        a,
        b,
        c,
        rOut: new TwistPoint(rx, ry, rz, rt),
    };
}

/**
 * See the doubling algorithm for a=0 from "Faster Computation of the
 * Tate Pairing", http://arxiv.org/pdf/0904.0854v3.pdf
 */
function lineFunctionDouble(r: TwistPoint, q: CurvePoint): Result {
    const A = r.getX().square();
    const B = r.getY().square();
    const C = B.square();
    let D = r.getX().add(B).square().sub(A).sub(C);
    D = D.add(D);

    const E = A.add(A).add(A);
    const G = E.square();

    let rx = G.sub(D).sub(D)
    let rz = r.getY().add(r.getZ()).square().sub(B).sub(r.getT());
    let ry = D.sub(rx).mul(E);

    let t = C.add(C)
    t = t.add(t);
    t = t.add(t);
    ry = ry.sub(t);

    let rt = rz.square();

    t = E.mul(r.getT());
    t = t.add(t);
    const b = GfP2.zero().sub(t).mulScalar(q.getX());
    t = B.add(B)
    t = t.add(t)
    const a = r.getX().add(E).square().sub(A).sub(G).sub(t);
    let c = rz.mul(r.getT());
    c = c.add(c).mulScalar(q.getY());

    return {
        a,
        b,
        c,
        rOut: new TwistPoint(rx, ry, rz, rt),
    };
}

function mulLine(ret: GfP12, res: Result): GfP12 {
    const a2 = new GfP6(GfP2.zero(), res.a, res.b).mul(ret.getX());
    const t3 = ret.getY().mulScalar(res.c);

    const t = res.b.add(res.c);
    const t2 = new GfP6(GfP2.zero(), res.a, t);
    
    let tx = ret.getX().add(ret.getY()).mul(t2).sub(a2).sub(t3);
    let ty = t3.add(a2.mulTau());

    return new GfP12(tx, ty);
}

/**
 * miller implements the Miller loop for calculating the Optimal Ate pairing.
 * See algorithm 1 from http://cryptojedi.org/papers/dclxvi-20100714.pdf
 */
function miller(q: TwistPoint, p: CurvePoint): GfP12 {
    let ret = GfP12.one();

    const aAffine = q.clone();
    aAffine.makeAffine();

    const bAffine = p.clone();
    bAffine.makeAffine();

    const minusA = new TwistPoint();
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
        } else if (sixuPlus2NAF[i - 1] == -1) {
            res = lineFunctionAdd(r, minusA, bAffine, r2);
        } else {
            continue;
        }

        ret = mulLine(ret, res);
        r = res.rOut;
    }

    const q1 = new TwistPoint(
        aAffine.getX().conjugate().mul(xiToPMinus1Over3),
        aAffine.getY().conjugate().mul(xiToPMinus1Over2),
        GfP2.one(),
        GfP2.one(),
    );

    const minusQ2 = new TwistPoint(
        aAffine.getX().mulScalar(new GfP(xiToPSquaredMinus1Over3)),
        aAffine.getY(),
        GfP2.one(),
        GfP2.one(),
    );

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
function finalExponentiation(a: GfP12): GfP12 {
    let t1 = a.conjugate();

    t1 = t1.mul(a.invert());
    const t2 = t1.frobeniusP2();
    t1 = t1.mul(t2);

    const fp = t1.frobenius();
    const fp2 = t1.frobeniusP2();
    const fp3 = fp2.frobenius();

    const fu = t1.exp(u);
    const fu2 = fu.exp(u);
    const fu3 = fu2.exp(u);
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
export function optimalAte(g1: G1, g2: G2): GT {
    const e = miller(g2.getPoint(), g1.getPoint());
    const ret = finalExponentiation(e);

    if (g1.isInfinity() || g2.isInfinity()) {
        return new GT(GfP12.one());
    }

    return new GT(ret);
}