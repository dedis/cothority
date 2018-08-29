package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.HashId;
import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import ch.epfl.dedis.lib.omniledger.darc.Signature;
import ch.epfl.dedis.proto.OmniLedgerProto;
import com.google.protobuf.ByteString;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;

/**
 * Implementation of {@link HashId}. This implementation is immutable and is can be used as key for collections
 */
public class InstanceId implements HashId {
    private final byte[] id;
    public final static int length = 32;

    public InstanceId(byte[] id) throws CothorityCryptoException {
        if (id.length != length) {
            throw new CothorityCryptoException("need 32 bytes for instanceID, only got " + id.length);
        }
        this.id = Arrays.copyOf(id, id.length);
    }

    public InstanceId(ByteString bs) throws CothorityCryptoException{
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
     * Creates an instance ID of all zeros, which is the place where the OmniLedger
     * config is stored.
     *
     * @return the zero instance ID
     */
    public static InstanceId zero() {
        byte[] z = new byte[length];
        try {
            return new InstanceId(z);
        } catch (CothorityCryptoException e) {
            // This "can't happen", since we know z is the right length.
            throw new RuntimeException(e);
        }
    }
}
