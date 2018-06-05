package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;
import com.google.protobuf.ByteString;

/**
 * Argument is used in an Instruction. It will end up as the input argument of the OmniLedger smart contract.
 */
public class Argument {
    private String name;
    private byte[] value;

    /**
     * Constructor for the argument.
     * @param name The name of the argument.
     * @param value The value of the argument.
     */
    public Argument(String name, byte[] value) {
        this.name = name;
        this.value = value;
    }

    /**
     * Getter for the name.
     * @return The name.
     */
    public String getName() {
        return name;
    }

    /**
     * Getter for the value.
     * @return The value.
     */
    public byte[] getValue() {
        return value;
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public TransactionProto.Argument toProto() {
        TransactionProto.Argument.Builder b = TransactionProto.Argument.newBuilder();
        b.setName(this.name);
        b.setValue(ByteString.copyFrom(this.value));
        return b.build();
    }
}
