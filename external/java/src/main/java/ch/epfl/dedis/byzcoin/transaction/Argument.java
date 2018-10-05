package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.ByteString;

import java.util.ArrayList;
import java.util.List;

/**
 * Argument is used in an Instruction. It will end up as the input argument of the smart contract.
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
     * Constructor from protobuf.
     * @param proto
     */
    public Argument(ByzCoinProto.Argument proto) {
        name = proto.getName();
        value = proto.getValue().toByteArray();
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
    public ByzCoinProto.Argument toProto() {
        ByzCoinProto.Argument.Builder b = ByzCoinProto.Argument.newBuilder();
        b.setName(this.name);
        b.setValue(ByteString.copyFrom(this.value));
        return b.build();
    }

    public static List<Argument> NewList(String key, byte[] value){
        List<Argument> ret = new ArrayList<>();
        ret.add(new Argument(key, value));
        return ret;
    }

    public static List<Argument> NewList(String key, String value) {
        return NewList(key, value.getBytes());
    }
}
