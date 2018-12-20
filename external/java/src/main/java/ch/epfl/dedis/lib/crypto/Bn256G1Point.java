package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.crypto.bn256.BN;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

import java.math.BigInteger;
import java.util.Arrays;

/**
 * The point used as a public key for a Bn256 signature
 */
public class Bn256G1Point implements Point {
    static final byte[] marshalID = "bn256.g1".getBytes();

    BN.G1 g1;

    /**
     * Create a point from the hexadecimal string representation
     * @param pubkey - string of the public key in hexadecimal
     */
    Bn256G1Point(String pubkey) {
        g1 = new BN.G1();
        g1.unmarshal(Hex.parseHexBinary(pubkey));
    }

    /**
     * Create a point from the data in byte array
     * @param b - marshaling of the point
     */
    Bn256G1Point(byte[] b) {
        if (Arrays.equals(marshalID, Arrays.copyOfRange(b, 0, 8))) {
            b = Arrays.copyOfRange(b, 8, b.length);
        }
        g1 = new BN.G1();
        g1.unmarshal(b);
    }

    Bn256G1Point(BN.G1 g1) {
        this.g1 = g1;
    }

    Bn256G1Point(BigInteger scalar) {
        this.g1 = new BN.G1();
        this.g1.scalarBaseMul(scalar);
    }

    /**
     * Returns a hard copy of the point
     * @return the copy
     */
    @Override
    public Point copy() {
        return new Bn256G1Point(this.g1.marshal());
    }

    /**
     * Checks the equality of two points
     * @param other the other point
     * @return true when both are the same point
     */
    @Override
    public boolean equals(Object other) {
        if (!(other instanceof Bn256G1Point)) {
            return false;
        }
        return Arrays.equals(((Bn256G1Point)other).toBytes(), this.toBytes());
    }

    /**
     * Multiply the point by the given scalar
     * @param s the scalar
     * @return the result of the multiplication
     */
    @Override
    public Point mul(Scalar s) {
        BigInteger k = new BigInteger(1, s.getBigEndian());
        BN.G1 p = new BN.G1();
        p.scalarMul(this.g1, k);
        return new Bn256G1Point(p);
    }

    /**
     * Add the two points together
     * @param other the other point
     * @return the result of the addition
     */
    @Override
    public Point add(Point other) {
        if (!(other instanceof Bn256G1Point)) {
            throw new UnsupportedOperationException();
        }
        BN.G1 p = new BN.G1();
        p.add(this.g1, ((Bn256G1Point)other).g1);
        return new Bn256G1Point(p);
    }

    /**
     * Returns the protobuf representation of the point that is the tag
     * for the first 8 bytes and then the point as byte array
     * @return the byte string of the marshaling
     */
    @Override
    public ByteString toProto() {
        ByteString id = ByteString.copyFrom("bn256.pt".getBytes());
        return id.concat(ByteString.copyFrom(this.toBytes()));
    }

    /**
     * Marshals the point
     * @return the byte array
     */
    @Override
    public byte[] toBytes() {
        return this.g1.marshal();
    }

    /**
     * Returns true when the point is the zero value of the field
     * @return the result as boolean
     */
    @Override
    public boolean isZero() {
        return g1.isInfinity();
    }

    /**
     * Produces the negative version of the point
     * @return the negative of the point
     */
    @Override
    public Point negate() {
        BN.G1 p = new BN.G1();
        p.neg(this.g1);
        return new Bn256G1Point(p);
    }

    @Override
    public byte[] data() {
        return this.g1.marshal();
    }

    /**
     * Stringify the point using the hexadecimal shape
     * @return an hex string
     */
    @Override
    public String toString() {
        return this.g1.toString();
    }

    /**
     * Perform the pairing operation on this point and G2.
     *
     * @param g2 is a point on G2.
     * @return result of pairing.
     */
    public BN.GT pair(Bn256G2Point g2) {
        return BN.pair(this.g1, g2.g2);
    }
}
