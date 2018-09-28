package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.HashId;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

import java.util.Arrays;

/**
 * Implementation of {@link HashId}. This implementation is immutable and is can be used as key for collections
 */
public class ClientTransactionId implements HashId {
    private final byte[] id;
    public final static int length = 32;

    public ClientTransactionId(byte[] id) throws CothorityCryptoException {
        if (id.length != length) {
            throw new CothorityCryptoException("need 32 bytes for a Client Transaction ID, only got " + id.length);
        }
        this.id = Arrays.copyOf(id, id.length);
    }

    @Override
    public byte[] getId() {
        return Arrays.copyOf(id, id.length);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        return Arrays.equals(id, ((ClientTransactionId) o).id);
    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(id);
    }

    @Override
    public String toString(){
        return Hex.printHexBinary(id);
    }

    public ByteString toByteString(){
        return ByteString.copyFrom(id);
    }
}
