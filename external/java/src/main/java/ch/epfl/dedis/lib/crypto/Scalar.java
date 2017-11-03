package ch.epfl.dedis.lib.crypto;

import com.google.protobuf.ByteString;
import net.i2p.crypto.eddsa.EdDSAPrivateKey;
import net.i2p.crypto.eddsa.math.FieldElement;
import net.i2p.crypto.eddsa.math.ed25519.Ed25519ScalarOps;
import net.i2p.crypto.eddsa.spec.EdDSAPrivateKeySpec;

import javax.xml.bind.DatatypeConverter;
import java.util.Arrays;

public class Scalar {
    public FieldElement fieldElement;

    public Scalar(String str) {
        this(str, true);
    }

    public Scalar(String str, boolean reduce) {
        this(DatatypeConverter.parseHexBinary(str), reduce);
    }

    public Scalar(byte[] b) {
        this(b, true);
    }

    public Scalar(byte[] b, boolean reduce) {
        if (reduce) {
            byte[] reduced = new Ed25519ScalarOps().reduce(Arrays.copyOfRange(b, 0, 64));
            fieldElement = Ed25519.field.fromByteArray(reduced);
        } else {
            fieldElement = Ed25519.field.fromByteArray(b);
        }
    }

    public Scalar(FieldElement f) {
        fieldElement = f;
    }

    public String toString() {
        return DatatypeConverter.printHexBinary(getLittleEndian());
    }

    public ByteString toProto() {
        return ByteString.copyFrom(reduce().getLittleEndian());
    }

    public byte[] toBytes() {
        return reduce().getLittleEndian();
    }

    public Scalar reduce() {
        return new Scalar(Ed25519.ed25519.getScalarOps().reduce(getLittleEndianFull()));
    }

    public Scalar copy() {
        return new Scalar(getLittleEndian());
    }

    public boolean equals(Scalar other) {
        return Arrays.equals(fieldElement.toByteArray(), other.fieldElement.toByteArray());
    }

    public Scalar addOne() {
        return new Scalar(fieldElement.addOne());
    }

    public byte[] getBigEndian() {
        return Ed25519.reverse(getLittleEndian());
    }

    public byte[] getLittleEndian() {
        return fieldElement.toByteArray();
    }

    public byte[] getLittleEndianFull() {
        return Arrays.copyOfRange(getLittleEndian(), 0, 64);
    }

    public Scalar add(Scalar b) {
        return new Scalar(fieldElement.add(b.fieldElement));
    }

    public Scalar sub(Scalar b) {
        return new Scalar(fieldElement.subtract(b.fieldElement));
    }

    public Scalar invert() {
        return new Scalar(fieldElement.invert());
    }

    public Scalar negate() {
        return Ed25519.prime_order.sub(this.reduce()).reduce();
    }

    public boolean isZero() {
        return !reduce().fieldElement.isNonZero();
    }

    public Point scalarMult(Point p) {
        if (p == null) {
            p = Ed25519.base;
        }
        return p.scalarMult(this);
    }

    public Scalar mul(Scalar s) {
        return new Scalar(Ed25519.ed25519.getScalarOps().multiplyAndAdd(fieldElement.toByteArray(), s.fieldElement.toByteArray(),
                Ed25519.field.ZERO.toByteArray()));
    }

    public EdDSAPrivateKey getPrivate() {
        EdDSAPrivateKeySpec spec = new EdDSAPrivateKeySpec(Ed25519.ed25519, getLittleEndianFull());
        return new EdDSAPrivateKey(spec);
    }
}
