package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.Sha256id;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

/**
 * This class represents a DarcId, which is the hash of the fixed fields.
 */
public class DarcId extends Sha256id {
    public DarcId(byte[] id) throws CothorityCryptoException{
        super(id);
    }
}
