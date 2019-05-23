package ch.epfl.dedis.lib.crypto.bn256;

import java.math.BigInteger;

class TwistPoint {
    GFp2 x, y, z, t;

    static GFp2 twistB = new GFp2(
            new BigInteger("6500054969564660373279643874235990574282535810762300357187714502686418407178"),
            new BigInteger("45500384786952622612957507119651934019977750675336102500314001518804928850249")
    );

    static TwistPoint twistGen = new TwistPoint(
            new GFp2(
                    new BigInteger("21167961636542580255011770066570541300993051739349375019639421053990175267184"),
                    new BigInteger("64746500191241794695844075326670126197795977525365406531717464316923369116492")
            ),
            new GFp2(
                    new BigInteger("20666913350058776956210519119118544732556678129809273996262322366050359951122"),
                    new BigInteger("17778617556404439934652658462602675281523610326338642107814333856843981424549")
            ),
            new GFp2(
                    new BigInteger("0"),
                    new BigInteger("1")
            ),
            new GFp2(
                    new BigInteger("0"),
                    new BigInteger("1")
            )
    );

    TwistPoint() {
        this.x = new GFp2();
        this.y = new GFp2();
        this.z = new GFp2();
        this.t = new GFp2();
    }

    TwistPoint(TwistPoint p) {
        this.x = new GFp2(p.x);
        this.y = new GFp2(p.y);
        this.z = new GFp2(p.z);
        this.t = new GFp2(p.t);
    }

    private TwistPoint(GFp2 x, GFp2 y, GFp2 z, GFp2 t) {
        this.x = x;
        this.y = y;
        this.z = z;
        this.t = t;
    }

    @Override
    public String toString() {
        TwistPoint c = new TwistPoint(this);
        return "(" + c.x.toString() + "," + c.y.toString() + "," + c.z.toString() + ")";
    }

    void set(TwistPoint a) {
        this.x.set(a.x);
        this.y.set(a.y);
        this.z.set(a.z);
        this.t.set(a.t);
    }

    boolean isOnCurve() {
        GFp2 yy = GFp2Pool.getInstance().get().square(this.y);
        GFp2 xxx = GFp2Pool.getInstance().get().square(this.x);
        xxx.mul(xxx, this.x);
        yy.sub(yy, xxx);
        yy.sub(yy, twistB);
        yy.minimal();
        boolean result = yy.x.signum() == 0 && yy.y.signum() == 0;
        GFp2Pool.getInstance().put(yy, xxx);
        return result;
    }

    void setInfinity() {
        this.z.setZero();
    }

    boolean isInfinity() {
        return this.z.isZero();
    }

    void add(TwistPoint a, TwistPoint b) {

        if (a.isInfinity()) {
            this.set(b);
            return;
        }
        if (b.isInfinity()) {
            this.set(a);
            return;
        }

        GFp2 z1z1 = GFp2Pool.getInstance().get().square(a.z);
        GFp2 z2z2 = GFp2Pool.getInstance().get().square(b.z);
        GFp2 u1 = GFp2Pool.getInstance().get().mul(a.x, z2z2);
        GFp2 u2 = GFp2Pool.getInstance().get().mul(b.x, z1z1);

        GFp2 t = GFp2Pool.getInstance().get().mul(b.z, z2z2);
        GFp2 s1 = GFp2Pool.getInstance().get().mul(a.y, t);

        t.mul(a.z, z1z1);
        GFp2 s2 = GFp2Pool.getInstance().get().mul(b.y, t);

        GFp2 h = GFp2Pool.getInstance().get().sub(u2, u1);
        GFp2Pool.getInstance().put(u2);

        boolean xEqual = h.isZero();

        t.add(h, h);
        GFp2 i = GFp2Pool.getInstance().get().square(t);
        GFp2 j = GFp2Pool.getInstance().get().mul(h, i);

        t.sub(s2, s1);
        GFp2Pool.getInstance().put(s2);

        boolean yEqual = t.isZero();
        if (xEqual && yEqual) {
            this.dbl(a);
            return;
        }
        GFp2 r = GFp2Pool.getInstance().get().add(t, t);

        GFp2 v = GFp2Pool.getInstance().get().mul(u1, i);

        GFp2Pool.getInstance().put(i, u1);


        GFp2 t4 = GFp2Pool.getInstance().get().square(r);
        t.add(v, v);
        GFp2 t6 = GFp2Pool.getInstance().get().sub(t4, j);
        this.x.sub(t6, t);

        t.sub(v, this.x);
        t4.mul(s1, j);

        t6.add(t4, t4);
        t4.mul(r, t);

        this.y.sub(t4, t6);

        t.add(a.z, b.z);
        t4.square(t);
        t.sub(t4, z1z1);

        t4.sub(t, z2z2);

        this.z.mul(t4, h);

        GFp2Pool.getInstance().put(s1, j, v, r, z1z1, z2z2, t, t4, t6, h);
    }

    void dbl(TwistPoint a) {
        GFp2 A = GFp2Pool.getInstance().get().square(a.x);
        GFp2 B = GFp2Pool.getInstance().get().square(a.y);
        GFp2 C = GFp2Pool.getInstance().get().square(B);

        t = GFp2Pool.getInstance().get().add(a.x, B);
        GFp2Pool.getInstance().put(B);

        GFp2 t2 = GFp2Pool.getInstance().get().square(t);
        t.sub(t2, A);
        t2.sub(t, C);
        GFp2 d = GFp2Pool.getInstance().get().add(t2, t2);
        t.add(A, A);
        GFp2 e = GFp2Pool.getInstance().get().add(t, A);
        GFp2Pool.getInstance().put(A);

        GFp2 f = GFp2Pool.getInstance().get().square(e);

        t.add(d, d);
        this.x.sub(f, t);

        t.add(C, C);
        t2.add(t, t);
        t.add(t2, t2);
        this.y.sub(d, this.x);
        t2.mul(e, this.y);
        this.y.sub(t2, t);

        t.mul(a.y, a.z);
        this.z.add(t, t);

        GFp2Pool.getInstance().put(C, t, t2, d, e, f);
    }

    TwistPoint mul(TwistPoint a, BigInteger scalar) {
        TwistPoint sum = new TwistPoint();
        sum.setInfinity();
        TwistPoint t = new TwistPoint();

        for (int i = scalar.bitLength(); i >= 0; i--) {
            t.dbl(sum);
            if (scalar.testBit(i)) {
                sum.add(t, a);
            } else {
                sum.set(t);
            }
        }

        this.set(sum);
        return this;
    }

    TwistPoint makeAffine() {
        if (this.z.isOne()) {
            return this;
        }
        if (this.isInfinity()) {
            this.x.setZero();
            this.y.setOne();
            this.z.setZero();
            this.t.setZero();
            return this;
        }

        GFp2 zInv = GFp2Pool.getInstance().get().invert(this.z);
        this.t = new GFp2().mul(this.y, zInv);
        GFp2 zInv2 = GFp2Pool.getInstance().get().square(zInv);
        this.y.mul(t, zInv2);
        this.t.mul(this.x, zInv2);
        this.x.set(this.t);
        this.z.setOne();
        this.t.setOne();

        GFp2Pool.getInstance().put(zInv, zInv2);

        return this;
    }

    void negative(TwistPoint a) {
        this.x.set(a.x);
        this.y.setZero();
        this.y.sub(this.y, a.y);
        this.z.set(a.z);
        this.t.setZero();
    }
}
