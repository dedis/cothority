package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;
import com.google.protobuf.ByteString;

public class Argument {
    private String name;
    private byte[] value;

    public Argument(String name, byte[] value) {
        this.name = name;
        this.value = value;
    }

    public String getName() {
        return name;
    }

    public byte[] getValue() {
        return value;
    }

    public TransactionProto.Argument toProto() {
        TransactionProto.Argument.Builder b = TransactionProto.Argument.newBuilder();
        b.setName(this.name);
        b.setValue(ByteString.copyFrom(this.value));
        return b.build();
    }
}
