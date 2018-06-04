package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;

import java.util.List;

/**
 * Spawn is an operation that an Instruction can take, it should be used for creating an object.
 */
public class Spawn {
    private String contractId;
    private List<Argument> arguments;

    public Spawn(String contractId, List<Argument> arguments) {
        this.contractId = contractId;
        this.arguments = arguments;
    }

    public String getContractId() {
        return contractId;
    }

    public List<Argument> getArguments() {
        return arguments;
    }

    public TransactionProto.Spawn toProto() {
        TransactionProto.Spawn.Builder b = TransactionProto.Spawn.newBuilder();
        b.setContractid(this.contractId);
        for (Argument a : this.arguments) {
            b.addArgs(a.toProto());
        }
        return b.build();
    }
}
