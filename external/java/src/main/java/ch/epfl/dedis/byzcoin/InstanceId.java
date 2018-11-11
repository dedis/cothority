package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.HashId;
import ch.epfl.dedis.lib.Hex;
import com.google.protobuf.ByteString;

import java.util.Arrays;

/**
 * Implementation of {@link HashId}. This implementation is immutable and is can be used as key for in the trie
 */
public class InstanceId implements HashId {
    private final byte[] id;
    public final static int length = 32;

    public InstanceId(byte[] id) {
        if (id.length != length) {
            throw new RuntimeException("need 32 bytes for instanceID, only got " + id.length);
        }
        this.id = Arrays.copyOf(id, id.length);
    }

    public InstanceId(ByteString bs) {
        this(bs.toByteArray());
    }

    @Override
    public byte[] getId() {
        return Arrays.copyOf(id, id.length);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        return Arrays.equals(id, ((InstanceId) o).id);
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

    /**
     * Creates a contract ID of all zeros, which is the place where the ByzCoin
     * config is stored.
     *
     * @return the zero instance ID
     */
    public static InstanceId zero() {
        byte[] z = new byte[length];
        return new InstanceId(z);
    }
}
