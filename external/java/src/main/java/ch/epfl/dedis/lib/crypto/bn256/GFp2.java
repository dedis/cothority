package ch.epfl.dedis.lib.crypto.bn256;

import java.math.BigInteger;

class GFp2 {
    BigInteger x, y;
    private static BigInteger p = Constants.p;

    GFp2() {
        this.x = BigInteger.ZERO;
        this.y = BigInteger.ZERO;
    }

    GFp2(GFp2 e) {
        this.x = e.x;
        this.y = e.y;
    }

    GFp2(BigInteger x, BigInteger y) {
        this.x = x;
        this.y = y;
    }

    @Override
    public String toString() {
        return "(" + this.x.mod(p).toString() + "," + this.y.mod(p).toString() + ")";
    }

    @Override
    public boolean equals(Object obj) {
        if (obj == null) {
            return false;
        }
        if (!(obj instanceof GFp2)) {
            return false;
        }
        GFp2 other = (GFp2)obj;
        this.minimal();
        other.minimal();
        return other.x.equals(this.x) && other.y.equals(this.y);
    }

    GFp2 set(GFp2 a) {
        this.x = a.x;
        this.y = a.y;
        return this;
    }

    GFp2 setZero() {
        this.x = BigInteger.ZERO;
        this.y = BigInteger.ZERO;
        return this;
    }

    GFp2 setOne() {
        this.x = BigInteger.ZERO;
        this.y = BigInteger.ONE;
        return this;
    }

    void minimal() {
        if (this.x.signum() < 0 || this.x.compareTo(p) >= 0) {
            this.x = this.x.mod(p);
        }
        if (this.y.signum() < 0 || this.y.compareTo(p) >= 0) {
            this.y = this.y.mod(p);
        }
    }

    boolean isZero() {
        return this.x.signum() == 0 && this.y.signum() == 0;
    }

    boolean isOne() {
        if (this.x.signum() != 0) {
            return false;
        }
        byte[] words = BN.bigIntegerToBytes(this.y);
        return words.length == 1 && words[0] == 1;
    }

    GFp2 conjugate(GFp2 a) {
        this.y = a.y;
        this.x = a.x.negate();
        return this;
    }

    GFp2 negative(GFp2 a) {
        this.x = a.x.negate();
        this.y = a.y.negate();
        return this;
    }

    GFp2 add(GFp2 a, GFp2 b) {
        this.x = a.x.add(b.x);
        this.y = a.y.add(b.y);
        return this;
    }

    GFp2 sub(GFp2 a, GFp2 b) {
        this.x = a.x.subtract(b.x);
        this.y = a.y.subtract(b.y);
        return this;
    }

    GFp2 dbl(GFp2 a) {
        this.x = a.x.shiftLeft(1);
        this.y = a.y.shiftLeft(1);
        return this;
    }

    GFp2 exp(GFp2 a, BigInteger power) {
        GFp2 sum = new GFp2();
        sum.setOne();
        GFp2 t = new GFp2();

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

    GFp2 mul(GFp2 a, GFp2 b) {
        BigInteger tx = a.x.multiply(b.y);
        BigInteger t = b.x.multiply(a.y);
        tx = tx.add(t).mod(p);

        BigInteger ty = a.y.multiply(b.y).mod(p);
        t = a.x.multiply(b.x).mod(p);
        ty = ty.subtract(t).mod(p);
        this.y = ty;
        this.x = tx;

        return this;
    }

    GFp2 mulScalar(GFp2 a, BigInteger b) {
        this.x = a.x.multiply(b);
        this.y = a.y.multiply(b);
        return this;
    }

    GFp2 mulXi(GFp2 a) {
        BigInteger tx = a.x.shiftLeft(1);
        tx = tx.add(a.x);
        tx = tx.add(a.y);

        BigInteger ty = a.y.shiftLeft(1);
        ty = ty.add(a.y);
        ty = ty.subtract(a.x);

        this.x = tx;
        this.y = ty;

        return this;
    }

    GFp2 square(GFp2 a) {
        BigInteger t1 = a.y.subtract(a.x);
        BigInteger t2 = a.x.add(a.y);
        BigInteger ty = t1.multiply(t2);
        ty = ty.mod(p);

        t1 = a.x.multiply(a.y);
        t1 = t1.shiftLeft(1);

        this.x = t1.mod(p);
        this.y = ty;

        return this;
    }

    GFp2 invert(GFp2 a) {
        BigInteger t = a.y.multiply(a.y).mod(p);
        BigInteger t2 = a.x.multiply(a.x).mod(p);
        t = t.add(t2);

        BigInteger inv = t.modInverse(p);

        this.x = a.x.negate();
        this.x = this.x.multiply(inv).mod(p);

        this.y = a.y.multiply(inv).mod(p);

        return this;
    }
}
