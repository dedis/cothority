package ch.epfl.dedis.lib.crypto.bn256;

class OptAte {
    private static class result {
        GFp2 a, b, c;
        TwistPoint rOut;

        result(GFp2 a, GFp2 b, GFp2 c, TwistPoint rOut) {
            this.a = a;
            this.b = b;
            this.c = c;
            this.rOut = rOut;
        }
    }

    private static result lineFunctionAdd(TwistPoint r, TwistPoint p, CurvePoint q, GFp2 r2) {
        GFp2 a, b, c;
        TwistPoint rOut;

        GFp2 B = GFpPool.getInstance().get2().mul(p.x, r.t);

        GFp2 D = GFpPool.getInstance().get2().add(p.y, r.z);
        D.square(D);
        D.sub(D, r2);
        D.sub(D, r.t);
        D.mul(D, r.t);

        GFp2 H = GFpPool.getInstance().get2().sub(B, r.x);
        GFpPool.getInstance().put2(B);

        GFp2 I = GFpPool.getInstance().get2().square(H);

        GFp2 E = GFpPool.getInstance().get2().add(I, I);
        E.add(E, E);

        GFp2 J = GFpPool.getInstance().get2().mul(H, E);

        GFp2 L1 = GFpPool.getInstance().get2().sub(D, r.y);
        GFpPool.getInstance().put2(D);

        L1.sub(L1, r.y);

        GFp2 V = GFpPool.getInstance().get2().mul(r.x, E);
        GFpPool.getInstance().put2(E);

        rOut = new TwistPoint();
        rOut.x.square(L1);
        rOut.x.sub(rOut.x, J);
        rOut.x.sub(rOut.x, V);
        rOut.x.sub(rOut.x, V);

        rOut.z.add(r.z, H);
        GFpPool.getInstance().put2(H);

        rOut.z.square(rOut.z);
        rOut.z.sub(rOut.z, r.t);
        rOut.z.sub(rOut.z, I);
        GFpPool.getInstance().put2(I);

        GFp2 t = GFpPool.getInstance().get2().sub(V, rOut.x);
        GFpPool.getInstance().put2(V);

        t.mul(t, L1);
        GFp2 t2 = GFpPool.getInstance().get2().mul(r.y, J);
        GFpPool.getInstance().put2(J);

        t2.add(t2, t2);
        rOut.y.sub(t, t2);

        rOut.t.square(rOut.z);

        t.add(p.y, rOut.z);
        t.square(t);
        t.sub(t, r2);
        t.sub(t, rOut.t);

        t2.mul(L1, p.x);
        t2.add(t2, t2);
        a = new GFp2();
        a.sub(t2, t);

        GFpPool.getInstance().put2(t, t2);

        c = new GFp2();
        c.mulScalar(rOut.z, q.y);
        c.add(c, c);

        b = new GFp2();
        b.setZero();
        b.sub(b, L1);
        GFpPool.getInstance().put2(L1);

        b.mulScalar(b, q.x);
        b.add(b, b);

        return new result(a, b, c, rOut);
    }

    private static result lineFunctionDouble(TwistPoint r, CurvePoint q) {
        GFp2 a, b, c;
        TwistPoint rOut;

        GFp2 A = GFpPool.getInstance().get2().square(r.x);
        GFp2 B = GFpPool.getInstance().get2().square(r.y);
        GFp2 C = GFpPool.getInstance().get2().square(B);

        GFp2 D = GFpPool.getInstance().get2().add(r.x, B);
        D.square(D);
        D.sub(D, A);
        D.sub(D, C);
        D.add(D, D);

        GFp2 E = GFpPool.getInstance().get2().add(A, A);
        E.add(E, A);

        GFp2 G = GFpPool.getInstance().get2().square(E);

        rOut = new TwistPoint();
        rOut.x.sub(G, D);
        rOut.x.sub(rOut.x, D);

        rOut.z.add(r.y, r.z);
        rOut.z.square(rOut.z);
        rOut.z.sub(rOut.z, B);
        rOut.z.sub(rOut.z, r.t);

        rOut.y.sub(D, rOut.x);
        rOut.y.mul(rOut.y, E);
        GFp2 t = GFpPool.getInstance().get2().add(C, C);
        t.add(t, t);
        t.add(t, t);
        rOut.y.sub(rOut.y, t);

        rOut.t.square(rOut.z);

        t.mul(E, r.t);
        t.add(t, t);
        b = new GFp2();
        b.setZero();
        b.sub(b, t);
        b.mulScalar(b, q.x);

        a = new GFp2();
        a.add(r.x, E);
        a.square(a);
        a.sub(a, A);
        a.sub(a, G);
        t.add(B, B);
        t.add(t, t);
        a.sub(a, t);

        GFpPool.getInstance().put2(A, B, C, D, E, G, t);

        c = new GFp2();
        c.mul(rOut.z, r.t);
        c.add(c, c);
        c.mulScalar(c, q.y);

        return new result(a, b, c, rOut);
    }

    private static void mulLine(GFp12 ret, GFp2 a, GFp2 b, GFp2 c) {
        GFp6 a2 = GFpPool.getInstance().get6();
        a2.x.setZero();
        a2.y.set(a);
        a2.z.set(b);
        a2.mul(a2, ret.x);
        GFp6 t3 = GFpPool.getInstance().get6().mulScalar(ret.y, c);

        GFp2 t = GFpPool.getInstance().get2();
        t.add(b, c);
        GFp6 t2 = GFpPool.getInstance().get6();
        t2.x.setZero();
        t2.y.set(a);
        t2.z.set(t);
        ret.x.add(ret.x, ret.y);

        ret.y.set(t3);

        ret.x.mul(ret.x, t2);
        ret.x.sub(ret.x, a2);
        ret.x.sub(ret.x, ret.y);
        a2.mulTau(a2);
        ret.y.add(ret.y, a2);

        GFpPool.getInstance().put2(t);
        GFpPool.getInstance().put6(a2, t3, t2);
    }

    private static byte[] sixuPlus2NAF = new byte[]{0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 0, -1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, -1, 0, 1, 0, 0, 0, 1, 0, -1, 0, 0, 0, -1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, -1, 0, -1, 0, 0, 0, 0, 1, 0, 0, 0, 1};

    private static GFp12 miller(TwistPoint q, CurvePoint p) {
        GFp12 ret = new GFp12(); // return
        ret.setOne();

        TwistPoint aAffine = new TwistPoint(GFpPool.getInstance());
        aAffine.set(q);
        aAffine.makeAffine();

        CurvePoint bAffine = new CurvePoint();
        bAffine.set(p);
        bAffine.makeAffine();

        TwistPoint minusA = new TwistPoint(GFpPool.getInstance());
        minusA.negative(aAffine);

        TwistPoint r = new TwistPoint(GFpPool.getInstance());
        r.set(aAffine);

        GFp2 r2 = GFpPool.getInstance().get2();
        r2.square(aAffine.y);

        for (int i = sixuPlus2NAF.length - 1; i > 0; i--) {
            result res = lineFunctionDouble(r, bAffine);
            GFp2 a = res.a;
            GFp2 b = res.b;
            GFp2 c = res.c;
            TwistPoint newR = res.rOut;
            if (i != sixuPlus2NAF.length - 1) {
                ret.square(ret);
            }

            mulLine(ret, a, b, c);
            r = newR;

            if (sixuPlus2NAF[i - 1] == 1) {
                result res1 = lineFunctionAdd(r, aAffine, bAffine, r2);
                a = res1.a;
                b = res1.b;
                c = res1.c;
                newR = res1.rOut;
            } else if (sixuPlus2NAF[i - 1] == -1) {
                result res2 = lineFunctionAdd(r, minusA, bAffine, r2);
                a = res2.a;
                b = res2.b;
                c = res2.c;
                newR = res2.rOut;
            } else {
                continue;
            }

            mulLine(ret, a, b, c);
            r = newR;
        }

        TwistPoint q1 = new TwistPoint(GFpPool.getInstance());
        q1.x.conjugate(aAffine.x);
        q1.x.mul(q1.x, Constants.xiToPMinus1Over3);
        q1.y.conjugate(aAffine.y);
        q1.y.mul(q1.y, Constants.xiToPMinus1Over2);
        q1.z.setOne();
        q1.t.setOne();

        TwistPoint minusQ2 = new TwistPoint(GFpPool.getInstance());
        minusQ2.x.mulScalar(aAffine.x, Constants.xiToPSquaredMinus1Over3);
        minusQ2.y.set(aAffine.y);
        minusQ2.z.setOne();
        minusQ2.t.setOne();

        r2.square(q1.y);
        result res = lineFunctionAdd(r, q1, bAffine, r2);
        GFp2 a = res.a;
        GFp2 b = res.b;
        GFp2 c = res.c;
        TwistPoint newR = res.rOut;
        mulLine(ret, a, b, c);
        r = newR;

        r2.square(minusQ2.y);
        result res2 = lineFunctionAdd(r, minusQ2, bAffine, r2);
        GFpPool.getInstance().put2(r2);

        a = res2.a;
        b = res2.b;
        c = res2.c;
        newR = res2.rOut;
        mulLine(ret, a, b, c);
        r = newR;

        // Free at the end as GFps are passed around.
        aAffine.free(GFpPool.getInstance());
        minusA.free(GFpPool.getInstance());
        r.free(GFpPool.getInstance());
        q1.free(GFpPool.getInstance());
        minusQ2.free(GFpPool.getInstance());

        return ret;
    }

    private static GFp12 finalExponentiation(GFp12 in) {
        GFp12 t1 = GFpPool.getInstance().get12();

        t1.x.negative(in.x);
        t1.y.set(in.y);

        GFp12 inv = GFpPool.getInstance().get12();
        inv.invert(in);
        t1.mul(t1, inv);

        GFpPool.getInstance().put12(inv);

        GFp12 t2 = GFpPool.getInstance().get12().frobeniusP2(t1);
        t1.mul(t1, t2);

        GFpPool.getInstance().put12(t2);

        GFp12 fp = GFpPool.getInstance().get12().frobenius(t1);
        GFp12 fp2 = GFpPool.getInstance().get12().frobeniusP2(t1);
        GFp12 fp3 = GFpPool.getInstance().get12().frobenius(fp2);

        GFp12 fu = GFpPool.getInstance().get12();
        GFp12 fu2 = GFpPool.getInstance().get12();
        GFp12 fu3 = GFpPool.getInstance().get12();
        fu.exp(t1, Constants.u);
        fu2.exp(fu, Constants.u);
        fu3.exp(fu2, Constants.u);

        GFp12 y3 = GFpPool.getInstance().get12().frobenius(fu);
        GFp12 fu2p = GFpPool.getInstance().get12().frobenius(fu2);
        GFp12 fu3p = GFpPool.getInstance().get12().frobenius(fu3);
        GFp12 y2 = GFpPool.getInstance().get12().frobeniusP2(fu2);

        GFp12 y0 = GFpPool.getInstance().get12();
        y0.mul(fp, fp2);
        y0.mul(y0, fp3);

        GFpPool.getInstance().put12(fp, fp2, fp3);

        GFp12 y1 = GFpPool.getInstance().get12();
        GFp12 y4 =GFpPool.getInstance().get12();
        GFp12 y5 = GFpPool.getInstance().get12();
        y1.conjugate(t1);
        y5.conjugate(fu2);
        y3.conjugate(y3);
        y4.mul(fu, fu2p);
        y4.conjugate(y4);

        GFpPool.getInstance().put12(fu2p, fu, fu2);

        GFp12 y6 = GFpPool.getInstance().get12();
        y6.mul(fu3, fu3p);
        y6.conjugate(y6);

        GFpPool.getInstance().put12(fu3p, fu3);

        GFp12 t0 = new GFp12();
        t0.square(y6);
        t0.mul(t0, y4);
        t0.mul(t0, y5);
        t1.mul(y3, y5);
        t1.mul(t1, t0);
        t0.mul(t0, y2);
        t1.square(t1);
        t1.mul(t1, t0);
        t1.square(t1);
        t0.mul(t1, y1);
        t1.mul(t1, y0);
        t0.square(t0);
        t0.mul(t0, t1);

        GFpPool.getInstance().put12(t1, y3, y2, y0, y1, y4, y5, y6);

        return t0;
    }

    static GFp12 optimalAte(TwistPoint a, CurvePoint b) {
        GFp12 e = miller(a, b);
        GFp12 ret = finalExponentiation(e);

        if (a.isInfinity() || b.isInfinity()) {
            ret.setOne();
        }

        return ret;
    }
}
