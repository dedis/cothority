package com.byzgen.ocsapi;

import javax.annotation.Nonnull;
import java.util.Arrays;

public final class UserId {
    final byte id[];

    public byte[] getId() {
        return Arrays.copyOf(id, id.length);
    }

    public UserId(@Nonnull UserId userId) {
        this(userId.id);
    }

    public UserId(@Nonnull byte id[]) {
        if (id.length != 32) {
            throw new IllegalArgumentException("Expected size of ID of user is 32 bytes");
        }
        this.id = Arrays.copyOf(id, id.length);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        UserId userId = (UserId) o;

        return Arrays.equals(id, userId.id);
    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(id);
    }
}
