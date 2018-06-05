package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;

/**
 * Delete is an operation that an Instruction can take, it should be used for deleting an object.
 */
public class Delete {
    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public TransactionProto.Delete toProto() {
        TransactionProto.Delete.Builder b = TransactionProto.Delete.newBuilder();
        return b.build();
    }
}
