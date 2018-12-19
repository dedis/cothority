package ch.epfl.dedis.lib.crypto.bn256;

import java.math.BigInteger;
import java.util.Arrays;
import java.util.Random;

public class BN {
    /**
     * The pair of a G1 point and a scalar that is used to generate it.
     */
    public static class PairG1 {
        public BigInteger k;
        public G1 p;

        public PairG1(BigInteger k, G1 p) {
            this.k = k;
            this.p = p;
        }
    }

    /**
     * The pair of a G2 point and a scalar that is used to generate it.
     */
    public static class PairG2 {
        public BigInteger k;
        public G2 p;

        public PairG2(BigInteger k, G2 p) {
            this.k = k;
            this.p = p;
        }
    }

    /**
     * A point in G1. This object is <em>not</em> thread-safe.
     */
    public static class G1 {
        CurvePoint p;

        /**
         * Construct a G1 point. There is no guarantee on its value, please set it later.
         */
        public G1() {
            this.p = new CurvePoint();
        }

        /**
         * Construct a G1 point using a given curve point.
         *
         * @param p is the curve point.
         */
        public G1(CurvePoint p) {
            this.p = p;
        }

        /**
         * Copy constructor for G1.
         *
         * @param p point to copy.
         */
        public G1(G1 p) {
            this.p = new CurvePoint(p.p);
        }

        /**
         * Construct a point for a scalar.
         *
         * @param k is the scalar that is multiplied to the generator point to create the object.
         */
        public G1(BigInteger k) {
            this.p = new CurvePoint().mul(CurvePoint.curveGen, k);
        }

        /**
         * Generate a random pair of a point and a scalar that is used to create the point.
         *
         * @param rnd is the random source.
         * @return a pair of a point and a scalar.
         */
        public static PairG1 rand(Random rnd) {
            BigInteger k = randPosBigInt(rnd, Constants.order);
            G1 p = new G1().scalarBaseMul(k);
            return new PairG1(k, p);
        }

        @Override
        public String toString() {
            return "bn256.G1" + this.p.toString();
        }

        /**
         * Perform a scalar multiplication with the generator point.
         *
         * @param k is the scalar.
         * @return the result which is also this object.
         */
        public G1 scalarBaseMul(BigInteger k) {
            this.p.mul(CurvePoint.curveGen, k);
            return this;
        }

        /**
         * Perform a scalar multiplication.
         *
         * @param a is the point.
         * @param k is the scalar.
         * @return the result which is also this object.
         */
        public G1 scalarMul(G1 a, BigInteger k) {
            this.p.mul(a.p, k);
            return this;
        }

        /**
         * Perform a point addition.
         *
         * @param a is a point.
         * @param b is a point.
         * @return the resulting point which is also this object.
         */
        public G1 add(G1 a, G1 b) {
            this.p.add(a.p, b.p);
            return this;
        }

        /**
         * Perform a point negation.
         *
         * @param a is the point for negation.
         * @return the resulting point which is also this object.
         */
        public G1 neg(G1 a) {
            this.p.negative(a.p);
            return this;
        }

        /**
         * Turns the point into its byte representation.
         *
         * @return the marshalled bytes.
         */
        public byte[] marshal() {
            final int numBytes = 256 / 8;

            // operate on a copy so that we do not modify the underlying curve during marshal
            BN.G1 c = new BN.G1(this);

            if (c.p.isInfinity()) {
                return new byte[numBytes * 2];
            }

            c.p.makeAffine();

            byte[] xBytes = bigIntegerToBytes(c.p.x.mod(Constants.p));
            byte[] yBytes = bigIntegerToBytes(c.p.y.mod(Constants.p));

            byte[] ret = new byte[numBytes * 2];
            System.arraycopy(xBytes, 0, ret, 1 * numBytes - xBytes.length, xBytes.length);
            System.arraycopy(yBytes, 0, ret, 2 * numBytes - yBytes.length, yBytes.length);

            return ret;
        }

        /**
         * Turns the byte representation to the point.
         *
         * @param m the input bytes.
         * @return an unmarshalled point when successful, otherwise null.
         */
        public G1 unmarshal(byte[] m) {
            final int numBytes = 256 / 8;

            if (m.length != 2 * numBytes) {
                return null;
            }

            this.p.x = new BigInteger(1, Arrays.copyOfRange(m, 0 * numBytes, 1 * numBytes));
            this.p.y = new BigInteger(1, Arrays.copyOfRange(m, 1 * numBytes, 2 * numBytes));

            if (this.p.x.signum() == 0 && this.p.y.signum() == 0) {
                this.p.y = BigInteger.ONE;
                this.p.z = BigInteger.ZERO;
                this.p.t = BigInteger.ZERO;
            } else {
                this.p.z = BigInteger.ONE;
                this.p.t = BigInteger.ONE;
                if (!this.p.isOnCurve()) {
                    return null;
                }
            }

            return this;
        }
    }

    /**
     * A point in G2. This object is <em>not</em> thread-safe.
     */
    public static class G2 {
        TwistPoint p;

        /**
         * Construct a G2 point. We make no guarantee on its value, please set it later.
         */
        public G2() {
            this.p = new TwistPoint();
        }

        /**
         * Construct a G2 point from an existing TwistPoint.
         *
         * @param p is the twist point.
         */
        public G2(TwistPoint p) {
            this.p = p;
        }

        /**
         * Copy construct for G2.
         *
         * @param p is G2 point to be copied.
         */
        public G2(G2 p) {
            this.p = new TwistPoint(p.p);
        }

        /**
         * Construct a point for a scalar.
         *
         * @param k is the scalar that is multiplied to the generator point to create the object.
         */
        public G2(BigInteger k) {
            this.p = new TwistPoint().mul(TwistPoint.twistGen, k);
        }

        /**
         * Generate a random pair of a point and a scalar that is used to create the point.
         *
         * @param rnd is the random source.
         * @return a pair of a point and a scalar.
         */
        public static PairG2 rand(Random rnd) {
            BigInteger k = randPosBigInt(rnd, Constants.order);
            G2 p = new G2().scalarBaseMul(k);
            return new PairG2(k, p);
        }

        @Override
        public String toString() {
            return "bn256.G2" + this.p.toString();
        }

        /**
         * Perform a scalar multiplication with the generator point.
         *
         * @param k is the scalar.
         * @return the result which is also this object.
         */
        public G2 scalarBaseMul(BigInteger k) {
            this.p.mul(TwistPoint.twistGen, k);
            return this;
        }

        /**
         * Perform a scalar multiplication.
         *
         * @param a is the point.
         * @param k is the scalar.
         * @return the result which is also this object.
         */
        public G2 scalarMul(G2 a, BigInteger k) {
            this.p.mul(a.p, k);
            return this;
        }

        /**
         * Perform a point addition.
         *
         * @param a is a point.
         * @param b is a point.
         * @return the resulting point which is also this object.
         */
        public G2 add(G2 a, G2 b) {
            this.p.add(a.p, b.p);
            return this;
        }

        /**
         * Turns the point into its byte representation.
         *
         * @return the marshalled bytes.
         */
        public byte[] marshal() {
            final int numBytes = 256 / 8;

            if (this.p.isInfinity()) {
                return new byte[numBytes * 4];
            }

            // operate on a copy so that we do not modify the underlying curve during marshal
            BN.G2 c = new BN.G2(this);

            c.p.makeAffine();

            byte[] xxBytes = bigIntegerToBytes(c.p.x.x.mod(Constants.p));
            byte[] xyBytes = bigIntegerToBytes(c.p.x.y.mod(Constants.p));
            byte[] yxBytes = bigIntegerToBytes(c.p.y.x.mod(Constants.p));
            byte[] yyBytes = bigIntegerToBytes(c.p.y.y.mod(Constants.p));

            byte[] ret = new byte[numBytes * 4];
            System.arraycopy(xxBytes, 0, ret, 1 * numBytes - xxBytes.length, xxBytes.length);
            System.arraycopy(xyBytes, 0, ret, 2 * numBytes - xyBytes.length, xyBytes.length);
            System.arraycopy(yxBytes, 0, ret, 3 * numBytes - yxBytes.length, yxBytes.length);
            System.arraycopy(yyBytes, 0, ret, 4 * numBytes - yyBytes.length, yyBytes.length);

            return ret;
        }

        /**
         * Turns the byte representation to the point.
         *
         * @param m the input bytes.
         * @return an unmarshalled point when successful, otherwise null.
         */
        public G2 unmarshal(byte[] m) {
            final int numBytes = 256 / 8;

            if (m.length != 4 * numBytes) {
                return null;
            }

            if (this.p == null) {
                this.p = new TwistPoint();
            }

            this.p.x.x = new BigInteger(1, Arrays.copyOfRange(m, 0 * numBytes, 1 * numBytes));
            this.p.x.y = new BigInteger(1, Arrays.copyOfRange(m, 1 * numBytes, 2 * numBytes));
            this.p.y.x = new BigInteger(1, Arrays.copyOfRange(m, 2 * numBytes, 3 * numBytes));
            this.p.y.y = new BigInteger(1, Arrays.copyOfRange(m, 3 * numBytes, 4 * numBytes));

            if (this.p.x.x.signum() == 0 && this.p.x.y.signum() == 0 && this.p.y.x.signum() == 0 && this.p.y.y.signum() == 0) {
                this.p.y.setOne();
                this.p.z.setZero();
                this.p.t.setZero();
            } else {
                this.p.z.setOne();
                this.p.t.setOne();

                if (!this.p.isOnCurve()) {
                    return null;
                }
            }

            return this;
        }
    }

    /**
     * GT represents an element in the GT field. This object is <em>not</em> thread-safe.
     */
    public static class GT {
        GFp12 p;

        /**
         * Construct a new GT object, we make no guarantee on its value, please set it later.
         */
        public GT() {
            this.p = new GFp12();
        }

        /**
         * Construct a new GT object from a GFp12 object.
         *
         * @param p is the GFp12 object.
         */
        public GT(GFp12 p) {
            this.p = p;
        }

        @Override
        public String toString() {
            return "bn256.GT" + this.p.toString();
        }

        /**
         * Perform a scalar multiplication.
         *
         * @param a is the point.
         * @param k is the scalar.
         * @return the result which is also this object.
         */
        public GT scalarMul(GT a, BigInteger k) {
            this.p.exp(a.p, k);
            return this;
        }

        /**
         * Perform an addition.
         *
         * @param a is an element.
         * @param b is an element.
         * @return the resulting element which is also this object.
         */
        public GT add(GT a, GT b) {
            this.p.mul(a.p, b.p);
            return this;
        }

        /**
         * Perform a negation.
         *
         * @param a is the element for negation.
         * @return the resulting element which is also this object.
         */
        public GT neg(GT a) {
            this.p.invert(a.p);
            return this;
        }

        /**
         * Turns the element into its byte representation.
         *
         * @return the marshalled bytes.
         */
        public byte[] marshal() {
            this.p.minimal();

            byte[] xxxBytes = bigIntegerToBytes(this.p.x.x.x);
            byte[] xxyBytes = bigIntegerToBytes(this.p.x.x.y);
            byte[] xyxBytes = bigIntegerToBytes(this.p.x.y.x);
            byte[] xyyBytes = bigIntegerToBytes(this.p.x.y.y);
            byte[] xzxBytes = bigIntegerToBytes(this.p.x.z.x);
            byte[] xzyBytes = bigIntegerToBytes(this.p.x.z.y);
            byte[] yxxBytes = bigIntegerToBytes(this.p.y.x.x);
            byte[] yxyBytes = bigIntegerToBytes(this.p.y.x.y);
            byte[] yyxBytes = bigIntegerToBytes(this.p.y.y.x);
            byte[] yyyBytes = bigIntegerToBytes(this.p.y.y.y);
            byte[] yzxBytes = bigIntegerToBytes(this.p.y.z.x);
            byte[] yzyBytes = bigIntegerToBytes(this.p.y.z.y);

            final int numBytes = 256 / 8;

            byte[] ret = new byte[numBytes * 12];
            System.arraycopy(xxxBytes, 0, ret, 1 * numBytes - xxxBytes.length, xxxBytes.length);
            System.arraycopy(xxyBytes, 0, ret, 2 * numBytes - xxyBytes.length, xxyBytes.length);
            System.arraycopy(xyxBytes, 0, ret, 3 * numBytes - xyxBytes.length, xyxBytes.length);
            System.arraycopy(xyyBytes, 0, ret, 4 * numBytes - xyyBytes.length, xyyBytes.length);
            System.arraycopy(xzxBytes, 0, ret, 5 * numBytes - xzxBytes.length, xzxBytes.length);
            System.arraycopy(xzyBytes, 0, ret, 6 * numBytes - xzyBytes.length, xzyBytes.length);
            System.arraycopy(yxxBytes, 0, ret, 7 * numBytes - yxxBytes.length, yxxBytes.length);
            System.arraycopy(yxyBytes, 0, ret, 8 * numBytes - yxyBytes.length, yxyBytes.length);
            System.arraycopy(yyxBytes, 0, ret, 9 * numBytes - yyxBytes.length, yyxBytes.length);
            System.arraycopy(yyyBytes, 0, ret, 10 * numBytes - yyyBytes.length, yyyBytes.length);
            System.arraycopy(yzxBytes, 0, ret, 11 * numBytes - yzxBytes.length, yzxBytes.length);
            System.arraycopy(yzyBytes, 0, ret, 12 * numBytes - yzyBytes.length, yzyBytes.length);

            return ret;
        }

        /**
         * Turns the byte representation to the element.
         *
         * @param m the input bytes.
         * @return an unmarshalled element when successful, otherwise null.
         */
        public GT unmarshal(byte[] m) {
            final int numBytes = 256 / 8;

            if (m.length != 12 * numBytes) {
                return null;
            }

            if (this.p == null) {
                this.p = new GFp12();
            }

            this.p.x.x.x = new BigInteger(1, Arrays.copyOfRange(m, 0 * numBytes, 1 * numBytes));
            this.p.x.x.y = new BigInteger(1, Arrays.copyOfRange(m, 1 * numBytes, 2 * numBytes));
            this.p.x.y.x = new BigInteger(1, Arrays.copyOfRange(m, 2 * numBytes, 3 * numBytes));
            this.p.x.y.y = new BigInteger(1, Arrays.copyOfRange(m, 3 * numBytes, 4 * numBytes));
            this.p.x.z.x = new BigInteger(1, Arrays.copyOfRange(m, 4 * numBytes, 5 * numBytes));
            this.p.x.z.y = new BigInteger(1, Arrays.copyOfRange(m, 5 * numBytes, 6 * numBytes));
            this.p.y.x.x = new BigInteger(1, Arrays.copyOfRange(m, 6 * numBytes, 7 * numBytes));
            this.p.y.x.y = new BigInteger(1, Arrays.copyOfRange(m, 7 * numBytes, 8 * numBytes));
            this.p.y.y.x = new BigInteger(1, Arrays.copyOfRange(m, 8 * numBytes, 9 * numBytes));
            this.p.y.y.y = new BigInteger(1, Arrays.copyOfRange(m, 9 * numBytes, 10 * numBytes));
            this.p.y.z.x = new BigInteger(1, Arrays.copyOfRange(m, 10 * numBytes, 11 * numBytes));
            this.p.y.z.y = new BigInteger(1, Arrays.copyOfRange(m, 11 * numBytes, 12 * numBytes));

            return this;
        }
    }

    /**
     * Perform the pairing operation.
     *
     * @param g1 is the G1 point.
     * @param g2 is the G2 point.
     * @return the GT point.
     */
    public static GT pair(G1 g1, G2 g2) {
        return new GT(OptAte.optimalAte(g2.p, g1.p));
    }

    static BigInteger randPosBigInt(Random rnd, BigInteger n) {
        BigInteger r;
        do {
            r = new BigInteger(n.bitLength(), rnd);
        } while (r.signum() <= 0 || r.compareTo(n) >= 0);
        return r;
    }

    /**
     * We have to use this function instead of the BigInteger.toByteArray method because the latter might produce
     * a leading zero which is different from the Go implementation.
     */
    static byte[] bigIntegerToBytes(final BigInteger a) {
        byte[] bytes = a.toByteArray();
        if (bytes[0] == 0) {
            return Arrays.copyOfRange(bytes, 1, bytes.length);
        }
        return bytes;
    }
}
