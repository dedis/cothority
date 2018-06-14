package ch.epfl.dedis.lib.omniledger.darc;

import ch.epfl.dedis.lib.Sha256id;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

/**
 * This class represents a DarcId, which is the hash of the fixed fields.
 */
public class DarcId extends Sha256id {
    public DarcId(byte[] id) throws CothorityCryptoException{
        super(id);
    }

    public DarcId(ByteString id) throws CothorityCryptoException{
        this(id.toByteArray());
    }
}
