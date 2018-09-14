package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.proto.ByzCoinProto;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Invoke is an operation that an Instruction can take, it should be used for mutating an object.
 */
public class Invoke {
    private String command;
    private List<Argument> arguments;

    /**
     * Constructor for the invoke action.
     * @param command The command to invoke in the contract.
     * @param arguments The arguments for the contract.
     */
    public Invoke(String command, List<Argument> arguments) {
        this.command = command;
        this.arguments = arguments;
    }

    /**
     * Constructor from one name/value.
     * @param command
     * @param name
     * @param value
     */
    public Invoke(String command, String name, byte[] value){
        this(command, Arrays.asList(new Argument(name, value)));
    }

    /**
     * Constructo from protobuf.
     * @param proto
     */
    public Invoke(ByzCoinProto.Invoke proto) {
        command = proto.getCommand();
        arguments = new ArrayList<Argument>();
        for (ByzCoinProto.Argument a : proto.getArgsList()) {
            arguments.add(new Argument(a));
        }
    }

    /**
     * Getter for the command.
     * @return The command.
     */
    public String getCommand() {
        return command;
    }

    /**
     * Getter for the arguments
     * @return The arguments.
     */
    public List<Argument> getArguments() {
        return arguments;
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public ByzCoinProto.Invoke toProto() {
        ByzCoinProto.Invoke.Builder b = ByzCoinProto.Invoke.newBuilder();
        b.setCommand(this.command);
        for (Argument a : this.arguments) {
            b.addArgs(a.toProto());
        }
        return b.build();
    }
}
