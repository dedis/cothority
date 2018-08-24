package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

/**
 * This class represents a SkipblockId, which is a sha256-hash of
 * the static fields of the skipblock.
 */
public class SkipblockId extends Sha256id {
    public SkipblockId(byte[] id) throws CothorityCryptoException{
        super(id);
    }

    public SkipblockId(ByteString bs) throws CothorityCryptoException{
        this(bs.toByteArray());
    }
}
