package ch.epfl.dedis.lib;

import com.google.protobuf.ByteString;

/**
 * This class represents a SkipblockId, which is a sha256-hash of
 * the static fields of the skipblock.
 */
public class SkipblockId extends Sha256id {
    public SkipblockId(byte[] id) {
        super(id);
    }

    public SkipblockId(ByteString bs) {
        this(bs.toByteArray());
    }
}
