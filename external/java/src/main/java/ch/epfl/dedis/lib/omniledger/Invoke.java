package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;

import java.util.List;

/**
 * Invoke is an operation that an Instruction can take, it should be used for mutating an object.
 */
public class Invoke {
    private String command;
    private List<Argument> arguments;

    public Invoke(String command, List<Argument> arguments) {
        this.command = command;
        this.arguments = arguments;
    }

    public String getCommand() {
        return command;
    }

    public List<Argument> getArguments() {
        return arguments;
    }

    public TransactionProto.Invoke toProto() {
        TransactionProto.Invoke.Builder b = TransactionProto.Invoke.newBuilder();
        b.setCommand(this.command);
        for (Argument a : this.arguments) {
            b.addArgs(a.toProto());
        }
        return b.build();
    }
}
