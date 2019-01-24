package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

public interface Point {
    Point mul(Scalar s);
    Point add(Point other);
    boolean isZero();
    Point negate();
    byte[] data() throws CothorityCryptoException;
    Point copy();
    ByteString toProto();
    byte[] toBytes();
    Point getZero();

    @Override
    String toString();

    @Override
    boolean equals(Object other);
}
