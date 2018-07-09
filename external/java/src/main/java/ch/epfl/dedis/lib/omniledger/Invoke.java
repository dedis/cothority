package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.OmniLedgerProto;

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

    public Invoke(String command, String argument, byte[] value){
        this(command, Arrays.asList(new Argument(argument, value)));
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
    public OmniLedgerProto.Invoke toProto() {
        OmniLedgerProto.Invoke.Builder b = OmniLedgerProto.Invoke.newBuilder();
        b.setCommand(this.command);
        for (Argument a : this.arguments) {
            b.addArgs(a.toProto());
        }
        return b.build();
    }
}
