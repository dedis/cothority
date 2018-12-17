package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import com.google.protobuf.ByteString;
import sun.reflect.generics.reflectiveObjects.NotImplementedException;

import java.util.Arrays;

/**
 * The point used as a public key for a Bn256 signature
 */
public class Bn256G2Point implements Point {
    static final byte[] marshalID = "bn256.pt".getBytes();

    private String hex;

    /**
     * Create a point from the hexadecimal string representation
     * @param pubkey - string of the public key in hexadecimal
     */
    Bn256G2Point(String pubkey) {
        hex = pubkey.toUpperCase();
    }

    /**
     * Create a point from the data in byte array
     * @param b - marshaling of the point
     */
    Bn256G2Point(byte[] b) {
        if (Arrays.equals(marshalID, Arrays.copyOfRange(b, 0, 8))) {
            b = Arrays.copyOfRange(b, 8, b.length);
        }

        hex = Hex.printHexBinary(b);
    }

    /**
     * Returns a hard copy of the point
     * @return the copy
     */
    @Override
    public Point copy() {
        return new Bn256G2Point(hex);
    }

    /**
     * Checks the equality of two points
     * @param other the other point
     * @return true when both are the same point
     */
    @Override
    public boolean equals(Object other) {
        if (!(other instanceof Bn256G2Point)) {
            return false;
        }

        return other.toString().equals(toString());
    }

    /**
     * Multiply the point by the given scalar
     * @param s the scalar
     * @return the result of the multiplication
     */
    @Override
    public Point mul(Scalar s) {
        throw new NotImplementedException();
    }

    /**
     * Add the two points together
     * @param other the other point
     * @return the result of the addition
     */
    @Override
    public Point add(Point other) {
        throw new NotImplementedException();
    }

    /**
     * Returns the protobuf representation of the point that is the tag
     * for the first 8 bytes and then the point as byte array
     * @return the byte string of the marshaling
     */
    @Override
    public ByteString toProto() {
        ByteString id = ByteString.copyFrom("bn256.pt".getBytes());
        return id.concat(ByteString.copyFrom(toBytes()));
    }

    /**
     * Marshals the point
     * @return the byte array
     */
    @Override
    public byte[] toBytes() {
        return Hex.parseHexBinary(hex);
    }

    /**
     * Returns true when the point is the zero value of the field
     * @return the result as boolean
     */
    @Override
    public boolean isZero() {
        return false;
    }

    /**
     * Produces the negative version of the point
     * @return the negative of the point
     */
    @Override
    public Point negate() {
        throw new NotImplementedException();
    }

    @Override
    public byte[] data() {
        throw new NotImplementedException();
    }

    /**
     * Stringify the point using the hexadecimal shape
     * @return an hex string
     */
    @Override
    public String toString() {
        return hex;
    }
}
