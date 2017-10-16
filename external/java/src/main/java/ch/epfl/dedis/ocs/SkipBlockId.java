package ch.epfl.dedis.ocs;

import javax.annotation.Nonnull;

public interface SkipBlockId {
    /**
     * Return binary form of block ID used by skipchain
     * @return binary form of block ID
     */
    @Nonnull
    byte [] getId();
}