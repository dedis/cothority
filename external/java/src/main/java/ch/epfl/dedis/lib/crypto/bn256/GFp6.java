package ch.epfl.dedis.lib.crypto.bn256;

import java.math.BigInteger;

class GFp6 {
    GFp2 x, y, z;

    GFp6() {
        this.x = new GFp2();
        this.y = new GFp2();
        this.z = new GFp2();
    }

    GFp6(GFp6 a) {
        this.x = new GFp2(a.x);
        this.y = new GFp2(a.y);
        this.z = new GFp2(a.z);
    }

    @Override
    public String toString() {
        return "(" + this.x.toString() + "," + this.y.toString() + "," + this.z.toString() + ")";
    }

    @Override
    public boolean equals(Object obj) {
        if (obj == null) {
            return false;
        }
        if (!(obj instanceof GFp6)) {
            return false;
        }
        GFp6 other = (GFp6)obj;
        return other.x.equals(this.x) && other.y.equals(this.y) && other.z.equals(this.z);
    }

    GFp6 set(GFp6 a) {
        this.x.set(a.x);
        this.y.set(a.y);
        this.z.set(a.z);
        return this;
    }

    GFp6 setZero() {
        this.x.setZero();
        this.y.setZero();
        this.z.setZero();
        return this;
    }

    GFp6 setOne() {
        this.x.setZero();
        this.y.setZero();
        this.z.setOne();
        return this;
    }

    void minimal() {
        this.x.minimal();
        this.y.minimal();
        this.z.minimal();
    }

    boolean isZero() {
        return this.x.isZero() && this.y.isZero() && this.z.isZero();
    }

    boolean isOne() {
        return this.x.isZero() && this.y.isZero() && this.z.isOne();
    }

    GFp6 negative(GFp6 a) {
        this.x.negative(a.x);
        this.y.negative(a.y);
        this.z.negative(a.z);
        return this;
    }

    GFp6 frobenius(GFp6 a) {
        this.x.conjugate(a.x);
        this.y.conjugate(a.y);
        this.z.conjugate(a.z);

        this.x.mul(this.x, Constants.xiTo2PMinus2Over3);
        this.y.mul(this.y, Constants.xiToPMinus1Over3);
        return this;
    }

    GFp6 frobeniusP2(GFp6 a) {
        this.x.mulScalar(a.x, Constants.xiTo2PSquaredMinus2Over3);
        this.y.mulScalar(a.y, Constants.xiToPSquaredMinus1Over3);
        this.z.set(a.z);
        return this;
    }

    GFp6 add(GFp6 a, GFp6 b) {
        this.x.add(a.x, b.x);
        this.y.add(a.y, b.y);
        this.z.add(a.z, b.z);
        return this;
    }

    GFp6 sub(GFp6 a, GFp6 b) {
        this.x.sub(a.x, b.x);
        this.y.sub(a.y, b.y);
        this.z.sub(a.z, b.z);
        return this;
    }

    GFp6 dbl(GFp6 a) {
        this.x.dbl(a.x);
        this.y.dbl(a.y);
        this.z.dbl(a.z);
        return this;
    }

    GFp6 mul(GFp6 a, GFp6 b) {
        GFp2 v0 = GFp2Pool.getInstance().get();
        v0.mul(a.z, b.z);
        GFp2 v1 = GFp2Pool.getInstance().get();
        v1.mul(a.y, b.y);
        GFp2 v2 = GFp2Pool.getInstance().get();
        v2.mul(a.x, b.x);

        GFp2 t0 = GFp2Pool.getInstance().get();
        t0.add(a.x, a.y);
        GFp2 t1 = GFp2Pool.getInstance().get();
        t1.add(b.x, b.y);
        GFp2 tz = GFp2Pool.getInstance().get();
        tz.mul(t0, t1);

        tz.sub(tz, v1);
        tz.sub(tz, v2);
        tz.mulXi(tz);
        tz.add(tz, v0);

        t0.add(a.y, a.z);
        t1.add(b.y, b.z);
        GFp2 ty = GFp2Pool.getInstance().get();
        ty.mul(t0, t1);
        ty.sub(ty, v0);
        ty.sub(ty, v1);
        t0.mulXi(v2);
        ty.add(ty, t0);

        t0.add(a.x, a.z);
        t1.add(b.x, b.z);
        GFp2 tx = GFp2Pool.getInstance().get();
        tx.mul(t0, t1);
        tx.sub(tx, v0);
        tx.add(tx, v1);
        tx.sub(tx, v2);

        this.x.set(tx);
        this.y.set(ty);
        this.z.set(tz);

        GFp2Pool.getInstance().put(v0, v1, v2, t0, t1, tz, ty, tx);

        return this;
    }

    GFp6 mulScalar(GFp6 a, GFp2 b) {
        this.x.mul(a.x, b);
        this.y.mul(a.y, b);
        this.z.mul(a.z, b);
        return this;
    }

    GFp6 mulGFP(GFp6 a, BigInteger b) {
        this.x.mulScalar(a.x, b);
        this.y.mulScalar(a.y, b);
        this.z.mulScalar(a.z, b);
        return this;
    }

    GFp6 mulTau(GFp6 a) {
        GFp2 tz = GFp2Pool.getInstance().get();
        tz.mulXi(a.x);
        GFp2 ty = GFp2Pool.getInstance().get();
        ty.set(a.y);

        this.y.set(a.z);
        this.x.set(ty);
        this.z.set(tz);

        GFp2Pool.getInstance().put(ty, tz);
        return this;
    }

    GFp6 square(GFp6 a) {
        GFp2 v0 = GFp2Pool.getInstance().get().square(a.z);
        GFp2 v1 = GFp2Pool.getInstance().get().square(a.y);
        GFp2 v2 = GFp2Pool.getInstance().get().square(a.x);

        GFp2 c0 = GFp2Pool.getInstance().get().add(a.x, a.y);
        c0.square(c0);
        c0.sub(c0, v1);
        c0.sub(c0, v2);
        c0.mulXi(c0);
        c0.add(c0, v0);

        GFp2 c1 = GFp2Pool.getInstance().get().add(a.y, a.z);
        c1.square(c1);
        c1.sub(c1, v0);
        c1.sub(c1, v1);
        GFp2 xiV2 = GFp2Pool.getInstance().get().mulXi(v2);
        c1.add(c1, xiV2);

        GFp2 c2 =GFp2Pool.getInstance().get().add(a.x, a.z);
        c2.square(c2);
        c2.sub(c2, v0);
        c2.add(c2, v1);
        c2.sub(c2, v2);

        this.x.set(c2);
        this.y.set(c1);
        this.z.set(c0);

        GFp2Pool.getInstance().put(v0, v1, v2, xiV2, c0, c1, c2);

        return this;
    }

    GFp6 invert(GFp6 a) {
        GFp2 t1 = GFp2Pool.getInstance().get();

        GFp2 A = GFp2Pool.getInstance().get();
        A.square(a.z);
        t1.mul(a.x, a.y);
        t1.mulXi(t1);
        A.sub(A, t1);

        GFp2 B = GFp2Pool.getInstance().get();
        B.square(a.x);
        B.mulXi(B);
        t1.mul(a.y, a.z);
        B.sub(B, t1);

        GFp2 C = GFp2Pool.getInstance().get();
        C.square(a.y);
        t1.mul(a.x, a.z);
        C.sub(C, t1);

        GFp2 F = GFp2Pool.getInstance().get();
        F.mul(C, a.y);
        F.mulXi(F);
        t1.mul(A, a.z);
        F.add(F, t1);
        t1.mul(B, a.x);
        t1.mulXi(t1);
        F.add(F, t1);

        F.invert(F);

        this.x.mul(C, F);
        this.y.mul(B, F);
        this.z.mul(A, F);

        GFp2Pool.getInstance().put(t1, A, B, C, F);

        return this;
    }
}
