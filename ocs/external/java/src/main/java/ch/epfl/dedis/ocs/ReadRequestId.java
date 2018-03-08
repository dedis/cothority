package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

/**
 * This class represents a ReadRequestId, which is a skipblockId.
 */
public class ReadRequestId extends SkipblockId {
    public ReadRequestId(byte[] id) throws CothorityCryptoException{
        super(id);
    }
}
