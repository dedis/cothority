package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Invoke is an operation that an Instruction can take, it should be used for mutating an object.
 */
public class Invoke {
    private String contractID;
    private String command;
    private List<Argument> arguments;

    /**
     * Constructor for the invoke action.
     *
     * @param cID       is the contract ID
     * @param command   is the command to invoke in the contract.
     * @param arguments is the arguments for the contract.
     */
    public Invoke(String cID, String command, List<Argument> arguments) {
        this.contractID = cID;
        this.command = command;
        this.arguments = arguments;
    }

    /**
     * Constructor from one argName/value.
     *
     * @param cID     is the contract ID
     * @param command is the command
     * @param argName is the argument name
     * @param value   is the value
     */
    public Invoke(String cID, String command, String argName, byte[] value) {
        this(cID, command, Arrays.asList(new Argument(argName, value)));
    }

    /**
     * Constructor from protobuf.
     *
     * @param proto the input proto
     */
    public Invoke(ByzCoinProto.Invoke proto) {
        contractID = proto.getContractid();
        command = proto.getCommand();
        arguments = new ArrayList<>();
        for (ByzCoinProto.Argument a : proto.getArgsList()) {
            arguments.add(new Argument(a));
        }
    }

    /**
     * Getter for the contract ID.
     */
    public String getContractID() {
        return contractID;
    }

    /**
     * Getter for the command.
     *
     * @return The command.
     */
    public String getCommand() {
        return command;
    }

    /**
     * Getter for the arguments
     *
     * @return The arguments.
     */
    public List<Argument> getArguments() {
        return arguments;
    }

    /**
     * Converts this object to the protobuf representation.
     *
     * @return The protobuf representation.
     */
    public ByzCoinProto.Invoke toProto() {
        ByzCoinProto.Invoke.Builder b = ByzCoinProto.Invoke.newBuilder();
        b.setContractid(this.contractID);
        b.setCommand(this.command);
        for (Argument a : this.arguments) {
            b.addArgs(a.toProto());
        }
        return b.build();
    }

    @Override
    public String toString() {
        return "contractID: " + this.contractID + ", command: " + this.command + ", argument: <hidden>";
    }
}
