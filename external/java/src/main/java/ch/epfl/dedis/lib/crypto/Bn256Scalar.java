package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.bn256.BN;
import com.google.protobuf.ByteString;

import java.math.BigInteger;
import java.util.Arrays;

public class Bn256Scalar implements Scalar {
    private BigInteger x;

    Bn256Scalar(BigInteger x) {
        this.x = x;
    }
    Bn256Scalar(byte[] buf) {
        x = new BigInteger(1, buf);
    }
    public String toString() {
        return x.toString();
    }

    public ByteString toProto() {
        return ByteString.copyFrom(this.toBytes());
    }

    public byte[] toBytes() {
        return bigIntegerToBytes(this.x);
    }

    public Scalar reduce() {
        return new Bn256Scalar(this.x.mod(BN.order));
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
        return this.toBytes();
    }

    public byte[] getLittleEndian() {
        return getLittleEndianFull();
    }

    public byte[] getLittleEndianFull() {
        return Ed25519.reverse(getBigEndian());
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
        return new Bn256Scalar(this.x.modInverse(BN.order));
    }
    public Scalar negate() {
        return new Bn256Scalar(this.x.negate());
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
