package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;

public class Delete {
    public Delete() {

    }

    public TransactionProto.Delete toProto() {
        TransactionProto.Delete.Builder b = TransactionProto.Delete.newBuilder();
        return b.build();
    }
}
