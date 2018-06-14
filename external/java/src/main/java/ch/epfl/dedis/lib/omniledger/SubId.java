package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.HashId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

import javax.annotation.Nonnull;
import javax.xml.bind.DatatypeConverter;
import java.util.Arrays;

/**
 * Implementation of {@link HashId}. This implementation is immutable and is can be used as key for collections
 */
public class SubId implements HashId {
    private final byte[] id;
    public final static int length = 32;

    public SubId(byte[] id) throws CothorityCryptoException {
        if (id.length != length) {
            throw new CothorityCryptoException("need 32 bytes for subId, only got " + id.length);
        }
        this.id = Arrays.copyOf(id, id.length);
    }

    @Override
    @Nonnull
    public byte[] getId() {
        return Arrays.copyOf(id, id.length);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        return Arrays.equals(id, ((SubId) o).id);
    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(id);
    }

    @Override
    public String toString() {
        return DatatypeConverter.printHexBinary(id);
    }

    public static SubId zero() throws CothorityCryptoException {
        return new SubId(new byte[32]);
    }
}
