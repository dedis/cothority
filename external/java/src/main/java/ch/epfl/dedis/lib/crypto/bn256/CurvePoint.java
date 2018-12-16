package ch.epfl.dedis.lib.crypto.bn256;
import java.math.BigInteger;

class CurvePoint {
    BigInteger x, y, z, t;

    static BigInteger curveB = new BigInteger("3");
    static CurvePoint curveGen = new CurvePoint(
            BigInteger.ONE,
            new BigInteger("-2"),
            BigInteger.ONE,
            BigInteger.ONE);
    private static BigInteger p = Constants.p;

    CurvePoint() {
        this.x = BigInteger.ZERO;
        this.y = BigInteger.ZERO;
        this.z = BigInteger.ZERO;
        this.t = BigInteger.ZERO;
    }

    private CurvePoint(BigInteger x, BigInteger y, BigInteger z, BigInteger t) {
        this.x = x;
        this.y = y;
        this.z = z;
        this.t = t;
    }

    public String toString() {
        this.makeAffine();
        return "(" + this.x.toString() + "," + this.y.toString() + ")";
    }

    void set(CurvePoint a) {
        this.x = a.x;
        this.y = a.y;
        this.z = a.z;
        this.t = a.t;
    }

    boolean isOnCurve() {
        BigInteger yy = this.y.multiply(this.y);
        BigInteger xxx = this.x.multiply(this.x).multiply(this.x);
        yy = yy.subtract(xxx);
        yy = yy.subtract(curveB);
        if (yy.signum() < 0 || yy.compareTo(p) >= 0) {
            yy = yy.mod(p);
        }
        return yy.signum() == 0;
    }

    void setInfinity() {
        this.z = BigInteger.ZERO;
    }

    boolean isInfinity() {
        return this.z.signum() == 0;
    }

    void add(CurvePoint a, CurvePoint b) {
        if (a.isInfinity()) {
            this.set(b);
            return;
        }

        if (b.isInfinity()) {
            this.set(a);
            return;
        }

        BigInteger z1z1 = a.z.multiply(a.z).mod(p);
        BigInteger z2z2 = b.z.multiply(b.z).mod(p);
        BigInteger u1 = a.x.multiply(z2z2).mod(p);
        BigInteger u2 = b.x.multiply(z1z1).mod(p);

        BigInteger t = b.z.multiply(z2z2).mod(p);
        BigInteger s1 = a.y.multiply(t).mod(p);

        t = a.z.multiply(z1z1).mod(p);
        BigInteger s2 = b.y.multiply(t).mod(p);

        BigInteger h = u2.subtract(u1);
        boolean xEqual = h.signum() == 0;

        t  = h.add(h);
        BigInteger i = t.multiply(t).mod(p);
        BigInteger j = h.multiply(i).mod(p);

        t = s2.subtract(s1);
        boolean yEqual = t.signum() == 0;
        if (xEqual && yEqual) {
            this.dbl(a);
            return;
        }

        BigInteger r = t.add(t);
        BigInteger v = u1.multiply(i).mod(p);

        BigInteger t4 = r.multiply(r).mod(p);
        t = v.add(v);
        BigInteger t6 = t4.subtract(j);
        this.x = t6.subtract(t);

        t = v.subtract(this.x);
        t4 = s1.multiply(j).mod(p);
        t6 = t4.add(t4);
        t4 = r.multiply(t).mod(p);
        this.y = t4.subtract(t6);

        t = a.z.add(b.z);
        t4 = t.multiply(t).mod(p);
        t = t4.subtract(z1z1);
        t4 = t.subtract(z2z2);
        this.z = t4.multiply(h).mod(p);
    }

    void dbl(CurvePoint a) {
        BigInteger A = a.x.multiply(a.x).mod(p);
        BigInteger B = a.y.multiply(a.y).mod(p);
        BigInteger C = B.multiply(B).mod(p);

        BigInteger t = a.x.add(B);
        BigInteger t2 = t.multiply(t).mod(p);
        t = t2.subtract(A);
        t2 = t.subtract(C);
        BigInteger d = t2.add(t2);
        t = A.add(A);
        BigInteger e = t.add(A);
        BigInteger f = e.multiply(e).mod(p);

        t = d.add(d);
        this.x = f.subtract(t);

        t = C.add(C);
        t2 = t.add(t);
        t = t2.add(t2);
        this.y = d.subtract(this.x);
        t2 = e.multiply(this.y).mod(p);
        this.y = t2.subtract(t);

        t = a.y.multiply(a.z).mod(p);
        this.z = t.add(t);
    }

    CurvePoint mul(CurvePoint a, BigInteger scalar) {
        CurvePoint sum = new CurvePoint();
        sum.setInfinity();
        CurvePoint t = new CurvePoint();

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


    CurvePoint makeAffine() {
        byte[] words = this.z.toByteArray();
        if (words.length == 1 && words[0] == 1) {
            return this;
        }
        if (this.isInfinity()) {
            this.x = BigInteger.ZERO;
            this.y = BigInteger.ONE;
            this.z = BigInteger.ZERO;
            this.t = BigInteger.ZERO;
            return this;
        }

        BigInteger zInv = this.z.modInverse(p);
        BigInteger t = this.y.multiply(zInv).mod(p);
        BigInteger zInv2 = zInv.multiply(zInv).mod(p);
        this.y = t.multiply(zInv2).mod(p);
        t = this.x.multiply(zInv2).mod(p);
        this.x = t;
        this.z = BigInteger.ONE;
        this.t = BigInteger.ONE;

        return this;
    }

    void negative(CurvePoint a) {
        this.x = a.x;
        this.y = a.y.negate();
        this.z = a.z;
        this.t = BigInteger.ZERO;
    }
}
