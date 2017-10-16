package ch.epfl.dedis.ocs;

import java.util.Arrays;

/**
 * Implementation of {@link DocumentId}. This implementation is immutable and is can be used as key for collections
 */
final class DocumentIdImpl implements DocumentId {
    private final byte[] id;

    DocumentIdImpl(byte[] id) {
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

        return Arrays.equals(id, ((DocumentIdImpl) o).id);
    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(id);
    }
}
