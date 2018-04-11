package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;

/**
 * This class represents a SkipblockId, which is a sha256-hash of
 * the static fields of the skipblock.
 */
public class SkipblockId extends Sha256id {
    public SkipblockId(byte[] id) throws CothorityCryptoException {
        super(id);
    }
}
