package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.OmniLedgerProto;

/**
 * Delete is an operation that an Instruction can take, it should be used for deleting an object.
 */
public class Delete {
    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public OmniLedgerProto.Delete toProto() {
        OmniLedgerProto.Delete.Builder b = OmniLedgerProto.Delete.newBuilder();
        return b.build();
    }
}
