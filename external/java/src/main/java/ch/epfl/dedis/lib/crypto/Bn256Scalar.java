package ch.epfl.dedis.lib.crypto;

import com.google.protobuf.ByteString;

import java.math.BigInteger;

public class Bn256Scalar implements Scalar {
    private BigInteger x;

    Bn256Scalar(BigInteger x) {
        this.x = x;
    }
    public String toString() {
        return x.toString();
    }

    public ByteString toProto() {
        return ByteString.copyFrom(this.toBytes());
    }

    public byte[] toBytes() {
        return x.toByteArray();
    }

    public Scalar reduce() {
        // TODO
        return this;
    }

    public Scalar copy() {
        return new Bn256Scalar(x);
    }

    public boolean equals(Scalar other) {
        return convert(other).x.equals(this.x);
    }

    public Scalar addOne() {
        return new Bn256Scalar(this.x.add(BigInteger.ONE));
    }

    public byte[] getBigEndian() {
        return this.x.toByteArray();
    }

    public byte[] getLittleEndian() {
        return Ed25519.reverse(getBigEndian());
    }

    public byte[] getLittleEndianFull() {
        // tODO
        return null;
    }

    public Scalar add(Scalar b) {
        return new Bn256Scalar(this.x.add(convert(b).x));
    }

    public Scalar sub(Scalar b) {
        if (!(b instanceof Bn256Scalar)) {
            return this;
        }
        return new Bn256Scalar(this.x.subtract(convert(b).x));
    }

    public Scalar invert() {
        // TODO
        return null;
    }
    public Scalar negate() {
        // TODO
        return null;
    }

    public boolean isZero() {
        return this.x.equals(BigInteger.ZERO);
    }


    public Scalar mul(Scalar s) {
        return new Bn256Scalar(this.x.multiply(convert(s).x));
    }

    private static Bn256Scalar convert(Scalar s) {
        if (!(s instanceof Bn256Scalar)) {
            throw new IllegalArgumentException(String.format("Error thrown because you are trying to operate an Bn256 with a Scalar implementing class %s", s.getClass().getName()));
        }
        return (Bn256Scalar) s;
    }
}
