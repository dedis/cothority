package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.proto.ByzCoinProto;

/**
 * Delete is an operation that an Instruction can take, it should be used for deleting an object.
 */
public class Delete {
    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public ByzCoinProto.Delete toProto() {
        ByzCoinProto.Delete.Builder b = ByzCoinProto.Delete.newBuilder();
        return b.build();
    }

    /**
     * Constructor from protobuf.
     * @param proto
     */
    public Delete(ByzCoinProto.Delete proto) {
    }
}
