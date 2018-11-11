package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

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
     * @param proto the input protobuf
     */
    public Delete(ByzCoinProto.Delete proto) {
    }
}
