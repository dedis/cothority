package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

/**
 * This class represents a WriteRequestId, which is a skipblockId.
 */
public class WriteRequestId extends SkipblockId {
    public WriteRequestId(byte[] id) throws CothorityCryptoException{
        super(id);
    }
}
