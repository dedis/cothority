package com.byzgen.ocsapi;

import javax.annotation.Nonnull;

public interface HashId {
    /**
     * Return binary form of block ID used by skipchain
     * @return binary form of block ID
     */
    @Nonnull
    byte [] getId();
}