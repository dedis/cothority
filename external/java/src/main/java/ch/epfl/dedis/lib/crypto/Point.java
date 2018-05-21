package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

public interface Point {
    public Point copy();
    public boolean equals(Point other);

    public Point mul(Scalar s);
    public Point add(Point other);

    public ByteString toProto();
    public byte[] toBytes();
    public boolean isZero();
    public String toString();

    public Point negate();
    public byte[] data() throws CothorityCryptoException;
    // public static Point embed(byte[] data) throws CothorityCryptoException;
}
