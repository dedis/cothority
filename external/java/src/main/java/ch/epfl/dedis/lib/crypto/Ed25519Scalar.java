package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import com.google.protobuf.ByteString;
import net.i2p.crypto.eddsa.EdDSAPrivateKey;
import net.i2p.crypto.eddsa.math.FieldElement;
import net.i2p.crypto.eddsa.math.ed25519.Ed25519ScalarOps;
import net.i2p.crypto.eddsa.spec.EdDSAPrivateKeySpec;

import java.util.Arrays;

public class Ed25519Scalar implements Scalar {
    public FieldElement fieldElement;

    public Ed25519Scalar(String str) {
        this(str, true);
    }

    public Ed25519Scalar(String str, boolean reduce) {
        this(Hex.parseHexBinary(str), reduce);
    }

    public Ed25519Scalar(byte[] b) {
        this(b, true);
    }

    public Ed25519Scalar(byte[] b, boolean reduce) {
        if (reduce) {
            byte[] reduced = new Ed25519ScalarOps().reduce(Arrays.copyOfRange(b, 0, 64));
            fieldElement = Ed25519.field.fromByteArray(reduced);
        } else {
            fieldElement = Ed25519.field.fromByteArray(b);
        }
    }

    public Ed25519Scalar(FieldElement f) {
        fieldElement = f;
    }

    public String toString() {
        return Hex.printHexBinary(getLittleEndian());
    }

    public ByteString toProto() {
        return ByteString.copyFrom(reduce().getLittleEndian());
    }

    public byte[] toBytes() {
        return reduce().getLittleEndian();
    }

    public Scalar reduce() {
        return new Ed25519Scalar(Ed25519.ed25519.getScalarOps().reduce(getLittleEndianFull()));
    }

    public Scalar copy() {
        return new Ed25519Scalar(getLittleEndian());
    }

    public boolean equals(Scalar other) {
        Ed25519Scalar s = convert(other);
        return Arrays.equals(fieldElement.toByteArray(), s.fieldElement.toByteArray());
    }

    public Scalar addOne() {
        return new Ed25519Scalar(fieldElement.addOne());
    }

    public byte[] getBigEndian() {
        return Ed25519.reverse(getLittleEndian());
    }

    public byte[] getLittleEndian() {
        return fieldElement.toByteArray();
    }

    public Scalar add(Scalar b) {
        Ed25519Scalar other = convert(b);
        return new Ed25519Scalar(fieldElement.add(other.fieldElement));
    }

    public Scalar sub(Scalar b) {
        Ed25519Scalar other = convert(b);
        return new Ed25519Scalar(fieldElement.subtract(other.fieldElement));
    }

    public Scalar invert() {
        return new Ed25519Scalar(fieldElement.invert());
    }

    public Scalar negate() {
        return convert(Ed25519.prime_order).sub(this.reduce()).reduce();
    }

    public boolean isZero() {
        return !convert(reduce()).fieldElement.isNonZero();
    }

    public Scalar mul(Scalar s) {
        Ed25519Scalar other = convert(s);
        return new Ed25519Scalar(Ed25519.ed25519.getScalarOps().multiplyAndAdd(fieldElement.toByteArray(), other.fieldElement.toByteArray(),
                Ed25519.field.ZERO.toByteArray()));
    }

    public EdDSAPrivateKey getPrivate() {
        EdDSAPrivateKeySpec spec = new EdDSAPrivateKeySpec(Ed25519.ed25519, getLittleEndianFull());
        return new EdDSAPrivateKey(spec);
    }

    private static Ed25519Scalar convert(Scalar s) {
        if (!(s instanceof Ed25519Scalar)) {
            throw new IllegalArgumentException(String.format("Error thrown because you are trying to operate an Ed25519Scalar with a Scalar implementing class %s", s.getClass().getName()));
        }
        return (Ed25519Scalar) s;
    }

    /**
     * Sometimes the scalar is small and getLittleEndian returns fewer than 32 bytes. Which may be a problem for some
     * methods in this file. This method always fills the little-endian representation to its maximum length with zeros.
     */
    private byte[] getLittleEndianFull() {
        return Arrays.copyOfRange(getLittleEndian(), 0, 64);
    }

}
