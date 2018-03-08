package ch.epfl.dedis.lib;

import javax.annotation.Nonnull;

public interface HashId {
    /**
     * Return binary form of block getId used by skipchain
     * @return binary form of block getId
     */
    @Nonnull
    byte [] getId();
}