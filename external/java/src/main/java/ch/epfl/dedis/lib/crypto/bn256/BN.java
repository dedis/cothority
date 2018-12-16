package ch.epfl.dedis.lib.crypto.bn256;

import java.math.BigInteger;
import java.util.Arrays;
import java.util.Random;

public class BN {
    static BigInteger randPosBigInt(Random rnd, BigInteger n) {
        BigInteger r;
        do {
            r = new BigInteger(n.bitLength(), rnd);
        } while (r.signum() <= 0 || r.compareTo(n) >= 0);
        return r;
    }

    public static class PairG1 {
        public BigInteger k;
        public G1 p;
        public PairG1(BigInteger k, G1 p) {
            this.k = k;
            this.p = p;
        }
    }

    public static class PairG2 {
        public BigInteger k;
        public G2 p;
        public PairG2(BigInteger k, G2 p) {
            this.k = k;
            this.p = p;
        }
    }

    public static class G1 {
        CurvePoint p;
        public G1() {
            this.p = new CurvePoint();
        }

        public G1(CurvePoint p) {
            this.p = p;
        }

        public static PairG1 rand(Random rnd) {
            BigInteger k = randPosBigInt(rnd, Constants.order);
            G1 p = new G1().scalarBaseMul(k);
            return new PairG1(k, p);
        }

        public String toString() {
            return "bn256.G1" + this.p.toString();
        }

        public G1 scalarBaseMul(BigInteger k) {
            this.p.mul(CurvePoint.curveGen, k);
            return this;
        }

        public G1 scalarMul(G1 a, BigInteger k) {
            this.p.mul(a.p, k);
            return this;
        }

        public G1 add(G1 a, G1 b) {
            this.p.add(a.p, b.p);
            return this;
        }

        public G1 neg(G1 a) {
            this.p.negative(a.p);
            return this;
        }

        public byte[] marshal() {
            final int numBytes = 256/8;

            if (this.p.isInfinity()) {
                return new byte[numBytes*2];
            }

            this.p.makeAffine();

            byte[] xBytes = this.p.x.mod(Constants.p).toByteArray();
            byte[] yBytes = this.p.y.mod(Constants.p).toByteArray();

            byte[] ret = new byte[numBytes*2];
            System.arraycopy(xBytes, 0, ret, 1*numBytes-xBytes.length, xBytes.length);
            System.arraycopy(yBytes, 0, ret, 2*numBytes-yBytes.length, yBytes.length);

            return ret;
        }

        public G1 unmarshal(byte[] m) {
            final int numBytes = 256/8;

            if (m.length != 2*numBytes) {
                return null;
            }

            this.p.x = new BigInteger(Arrays.copyOfRange(m, 0*numBytes, 1*numBytes));
            this.p.y = new BigInteger(Arrays.copyOfRange(m, 1*numBytes, 2*numBytes));

            if (this.p.x.signum() == 0 && this.p.y.signum() == 0) {
                this.p.y = BigInteger.ONE;
                this.p.z = BigInteger.ZERO;
                this.p.t = BigInteger.ZERO;
            } else {
                this.p.z = BigInteger.ONE;
                this.p.t = BigInteger.ONE;
                if (!this.p.isOnCurve())  {
                    return null;
                }
            }

            return this;
        }
    }

    public static class G2 {
        TwistPoint p;
        public G2() {
            this.p = new TwistPoint();
        }

        public G2(TwistPoint p) {
            this.p = p;
        }

        public static PairG2 rand(Random rnd) {
            BigInteger k = randPosBigInt(rnd, Constants.order);
            G2 p = new G2().scalarBaseMul(k);
            return new PairG2(k, p);
        }

        public String toString() {
            return "bn256.G2" + this.p.toString();
        }

        public G2 scalarBaseMul(BigInteger k) {
            this.p.mul(TwistPoint.twistGen, k);
            return this;
        }

        public G2 sclarMul(G2 a, BigInteger k) {
            this.p.mul(a.p, k);
            return this;
        }

        public G2 add(G2 a, G2 b) {
            this.p.add(a.p, b.p);
            return this;
        }

        public byte[] marshal() {
            final int numBytes = 256/8;

            if (this.p.isInfinity()) {
                return new byte[numBytes*4];
            }

            this.p.makeAffine();

            byte[] xxBytes = this.p.x.x.mod(Constants.p).toByteArray();
            byte[] xyBytes = this.p.x.y.mod(Constants.p).toByteArray();
            byte[] yxBytes = this.p.y.x.mod(Constants.p).toByteArray();
            byte[] yyBytes = this.p.y.y.mod(Constants.p).toByteArray();

            byte[] ret = new byte[numBytes*4];
            System.arraycopy(xxBytes, 0, ret, 1*numBytes-xxBytes.length, xxBytes.length);
            System.arraycopy(xyBytes, 0, ret, 2*numBytes-xyBytes.length, xyBytes.length);
            System.arraycopy(yxBytes, 0, ret, 3*numBytes-yxBytes.length, yxBytes.length);
            System.arraycopy(yyBytes, 0, ret, 4*numBytes-yyBytes.length, yyBytes.length);

            return ret;
        }

        public G2 unmarshal(byte[] m) {
            // TODO
            return null;
        }
    }

    public static class GT {
        GFp12 p;
        public GT() {
            this.p = new GFp12();
        }
        public GT(GFp12 p) {
            this.p = p;
        }
        public String toString() {
            return "bn256.GT" + this.p.toString();
        }
        public GT scalarMul(GT a, BigInteger k) {
            this.p.exp(a.p, k);
            return this;
        }
        public GT add(GT a, GT b) {
            this.p.mul(a.p, b.p);
            return this;
        }
        public GT neg(GT a) {
            this.p.invert(a.p);
            return this;
        }
        public byte[] marshal() {
            this.p.minimal();

            byte[] xxxBytes = this.p.x.x.x.toByteArray();
            byte[] xxyBytes = this.p.x.x.y.toByteArray();
            byte[] xyxBytes = this.p.x.y.x.toByteArray();
            byte[] xyyBytes = this.p.x.y.y.toByteArray();
            byte[] xzxBytes = this.p.x.z.x.toByteArray();
            byte[] xzyBytes = this.p.x.z.y.toByteArray();
            byte[] yxxBytes = this.p.y.x.x.toByteArray();
            byte[] yxyBytes = this.p.y.x.y.toByteArray();
            byte[] yyxBytes = this.p.y.y.x.toByteArray();
            byte[] yyyBytes = this.p.y.y.y.toByteArray();
            byte[] yzxBytes = this.p.y.z.x.toByteArray();
            byte[] yzyBytes = this.p.y.z.y.toByteArray();

            final int numBytes = 256/8;

            byte[] ret = new byte[numBytes*12];
            System.arraycopy(xxxBytes, 0, ret, 1*numBytes-xxxBytes.length, xxxBytes.length);
            System.arraycopy(xxyBytes, 0, ret, 2*numBytes-xxyBytes.length, xxyBytes.length);
            System.arraycopy(xyxBytes, 0, ret, 3*numBytes-xyxBytes.length, xyxBytes.length);
            System.arraycopy(xyyBytes, 0, ret, 4*numBytes-xyyBytes.length, xyyBytes.length);
            System.arraycopy(xzxBytes, 0, ret, 5*numBytes-xzxBytes.length, xzxBytes.length);
            System.arraycopy(xzyBytes, 0, ret, 6*numBytes-xzyBytes.length, xzyBytes.length);
            System.arraycopy(yxxBytes, 0, ret, 7*numBytes-yxxBytes.length, yxxBytes.length);
            System.arraycopy(yxyBytes, 0, ret, 8*numBytes-yxyBytes.length, yxyBytes.length);
            System.arraycopy(yyxBytes, 0, ret, 9*numBytes-yyxBytes.length, yyxBytes.length);
            System.arraycopy(yyyBytes, 0, ret, 10*numBytes-yyyBytes.length, yyyBytes.length);
            System.arraycopy(yzxBytes, 0, ret, 11*numBytes-yzxBytes.length, yzxBytes.length);
            System.arraycopy(yzyBytes, 0, ret, 12*numBytes-yzyBytes.length, yzyBytes.length);
            return ret;
        }
        public GT unmarshal() {
            // TODO
            return null;
        }
    }

    public static GT pair(G1 g1, G2 g2) {
        return new GT(OptAte.optimalAte(g2.p, g1.p));
    }
}
