package ch.epfl.dedis.lib.crypto;

import com.google.protobuf.ByteString;

public interface Scalar {
    public String toString();
    public ByteString toProto();
    public byte[] toBytes();

    public Scalar reduce();
    public Scalar copy();

    public boolean equals(Scalar other);

    public Scalar addOne();

    public byte[] getBigEndian();
    public byte[] getLittleEndian();

    public Scalar add(Scalar b);
    public Scalar sub(Scalar b);
    public Scalar invert();
    public Scalar negate();
    public boolean isZero();

    public Scalar mul(Scalar s);
}
