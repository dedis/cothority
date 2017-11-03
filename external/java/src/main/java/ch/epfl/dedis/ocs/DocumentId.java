package ch.epfl.dedis.ocs;

import com.byzgen.ocsimpl.HashId;

import java.util.Arrays;

/**
 * Implementation of {@link HashId}. This implementation is immutable and is can be used as key for collections
 */
final class DocumentId implements HashId {
    private final byte[] id;

    DocumentId(byte[] id) {
        this.id = Arrays.copyOf(id, id.length);
    }

    @Override
    public byte[] getId() {
        return Arrays.copyOf(id, id.length);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        return Arrays.equals(id, ((DocumentId) o).id);
    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(id);
    }
}
