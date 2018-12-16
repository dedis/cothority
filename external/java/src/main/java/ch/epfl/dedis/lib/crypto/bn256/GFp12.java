package ch.epfl.dedis.lib.crypto.bn256;
import java.math.BigInteger;

class GFp12 {
    GFp6 x, y;

    GFp12() {
        this.x = new GFp6();
        this.y = new GFp6();
    }

    GFp12(GFp12 a) {
        this.x = new GFp6(a.x);
        this.y = new GFp6(a.y);
    }

    public String toString() {
        return "(" + this.x.toString() + "," + this.y.toString() + ")";
    }

    GFp12 set(GFp12 a) {
        this.x.set(a.x);
        this.y.set(a.y);
        return this;
    }

    GFp12 setZero() {
        this.x.setZero();
        this.y.setZero();
        return this;
    }

    GFp12 setOne() {
        this.x.setZero();
        this.y.setOne();
        return this;
    }

    void minimal() {
        this.x.minimal();
        this.y.minimal();
    }

    boolean isZero() {
        this.minimal();
        return this.x.isZero() && this.y.isZero();
    }

    boolean isOne() {
        this.minimal();
        return this.x.isZero() && this.y.isOne();
    }

    GFp12 conjugate(GFp12 a) {
        this.x.negative(a.x);
        this.y.set(a.y);
        return this;
    }

    GFp12 negative(GFp12 a) {
        this.x.negative(a.x);
        this.y.negative(a.y);
        return this;
    }

    GFp12 frobenius(GFp12 a) {
        this.x.frobenius(a.x);
        this.y.frobenius(a.y);
        this.x.mulScalar(this.x, Constants.xiToPMinus1Over6);
        return this;
    }

    GFp12 frobeniusP2(GFp12 a) {
        this.x.frobeniusP2(a.x);
        this.x.mulGFP(this.x, Constants.xiToPSquaredMinus1Over6);
        this.y.frobeniusP2(a.y);
        return this;
    }

    GFp12 add(GFp12 a, GFp12 b) {
        this.x.add(a.x, b.x);
        this.y.add(a.y, b.y);
        return this;
    }

    GFp12 sub(GFp12 a, GFp12 b) {
        this.x.sub(a.x, b.x);
        this.y.sub(a.y, b.y);
        return this;
    }

    GFp12 mul(GFp12 a, GFp12 b) {
        GFp6 tx = new GFp6();
        tx.mul(a.x, b.y);
        GFp6 t = new GFp6();
        t.mul(b.x, a.y);
        tx.add(tx, t);

        GFp6 ty = new GFp6();
        ty.mul(a.y, b.y);
        t.mul(a.x, b.x);
        t.mulTau(t);
        this.y.add(ty, t);
        this.x.set(tx);

        return this;
    }

    GFp12 mulScalar(GFp12 a, GFp6 b) {
        this.x.mul(a.x, b);
        this.y.mul(a.y, b);
        return this;
    }

    GFp12 exp(GFp12 a, BigInteger power) {
        GFp12 sum = new GFp12();
        sum.setOne();
        GFp12 t = new GFp12();

        for (int i = power.bitLength() - 1; i >= 0; i--) {
            t.square(sum);
            if (power.testBit(i)) {
                sum.mul(t, a);
            } else {
                sum.set(t);
            }
        }

        this.set(sum);
        return this;
    }

    GFp12 square(GFp12 a) {
        GFp6 v0 = new GFp6();
        v0.mul(a.x, a.y);

        GFp6 t = new GFp6();
        t.mulTau(a.x);
        t.add(a.y, t);
        GFp6 ty = new GFp6();
        ty.add(a.x, a.y);
        ty.mul(ty, t);
        ty.sub(ty, v0);
        t.mulTau(v0);
        ty.sub(ty, t);

        this.y.set(ty);
        this.x.dbl(v0);

        return this;
    }

    GFp12 invert(GFp12 a) {
        GFp6 t1 = new GFp6();
        GFp6 t2 = new GFp6();

        t1.square(a.x);
        t2.square(a.y);
        t1.mulTau(t1);
        t1.sub(t2, t1);
        t2.invert(t1);

        this.x.negative(a.x);
        this.y.set(a.y);
        this.mulScalar(this, t2);

        return this;
    }
}
