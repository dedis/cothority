package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;
import ch.epfl.dedis.lib.crypto.bn256.BN;

import java.math.BigInteger;
import java.util.Arrays;

/**
 * The point used as a public key for a Bn256 signature
 */
public class Bn256G2Point implements Point {
    static final byte[] marshalID = "bn256.g2".getBytes();

    BN.G2 g2;

    /**
     * Create a point by multiplying the base point with a scalar.
     *
     * @param k the scalar to be multiplied to the base point.
     */
    Bn256G2Point(BigInteger k) {
        g2 = new BN.G2();
        g2.scalarBaseMul(k);
    }

    /**
     * Create a point from the hexadecimal string representation
     *
     * @param pubkey - string of the public key in hexadecimal
     */
    Bn256G2Point(String pubkey) throws CothorityCryptoException {
        this(Hex.parseHexBinary(pubkey));
    }

    /**
     * Create a point from the data in byte array
     *
     * @param b - marshaling of the point
     */
    Bn256G2Point(byte[] b) throws CothorityCryptoException {
        if (Arrays.equals(marshalID, Arrays.copyOfRange(b, 0, 8))) {
            b = Arrays.copyOfRange(b, 8, b.length);
        }
        g2 = new BN.G2();
        if (g2.unmarshal(b) == null) {
            throw new CothorityCryptoException("invalid buffer");
        }
    }

    /**
     * Create the point from a BN.G2 point.
     *
     * @param g2 the G2 point.
     */
    Bn256G2Point(BN.G2 g2) {
        this.g2 = new BN.G2(g2);
    }

    /**
     * Returns a hard copy of the point
     *
     * @return the copy
     */
    @Override
    public Point copy() {
        return new Bn256G2Point(this.g2);
    }

    /**
     * Checks the equality of two points
     *
     * @param other the other point
     * @return true when both are the same point
     */
    @Override
    public boolean equals(Object other) {
        if (!(other instanceof Bn256G2Point)) {
            return false;
        }
        // TODO not super efficient, consider adding an equals method to BN.G2 that checks the underlying bigints.
        return Arrays.equals(((Bn256G2Point) other).toBytes(), this.toBytes());
    }

    /**
     * Multiply the point by the given scalar
     *
     * @param s the scalar
     * @return the result of the multiplication
     */
    @Override
    public Point mul(Scalar s) {
        if (!(s instanceof Bn256Scalar)) {
            throw new UnsupportedOperationException();
        }
        BigInteger k = new BigInteger(1, s.getBigEndian());
        BN.G2 p = new BN.G2();
        p.scalarMul(this.g2, k);
        return new Bn256G2Point(p);
    }

    /**
     * Add the two points together
     *
     * @param other the other point
     * @return the result of the addition
     */
    @Override
    public Point add(Point other) {
        if (!(other instanceof Bn256G2Point)) {
            throw new UnsupportedOperationException();
        }
        BN.G2 p = new BN.G2();
        p.add(this.g2, ((Bn256G2Point) other).g2);
        return new Bn256G2Point(p);
    }

    /**
     * Returns the protobuf representation of the point that is the tag
     * for the first 8 bytes and then the point as byte array
     *
     * @return the byte string of the marshaling
     */
    @Override
    public ByteString toProto() {
        ByteString id = ByteString.copyFrom(marshalID);
        return id.concat(ByteString.copyFrom(this.toBytes()));
    }

    /**
     * Marshals the point
     *
     * @return the byte array
     */
    @Override
    public byte[] toBytes() {
        return this.g2.marshal();
    }

    /**
     * Returns true when the point is the zero value of the field
     *
     * @return the result as boolean
     */
    @Override
    public boolean isZero() {
        return g2.isInfinity();
    }

    /**
     * Produces the negative version of the point
     *
     * @return the negative of the point
     */
    @Override
    public Point negate() {
        BN.G2 p = new BN.G2();
        p.neg(this.g2);
        return new Bn256G2Point(p);
    }

    @Override
    public byte[] data() {
        return this.g2.marshal();
    }

    /**
     * Stringify the point using the hexadecimal shape
     *
     * @return an hex string
     */
    @Override
    public String toString() {
        return this.g2.toString();
    }

    /**
     * Get the infinity (zero) point.
     */
    @Override
    public Point getZero() {
        BN.G2 p = new BN.G2();
        p.setInfinity();
        return new Bn256G2Point(p);
    }

    /**
     * Perform the pairing operation on this point and G1.
     *
     * @param g1 is a point on G1.
     * @return result of pairing.
     */
    public BN.GT pair(Bn256G1Point g1) {
        return BN.pair(g1.g1, this.g2);
    }

}
