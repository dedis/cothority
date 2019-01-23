package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

import java.util.ArrayList;
import java.util.List;

/**
 * Spawn is an operation that an Instruction can take, it should be used for creating an object.
 */
public class Spawn {
    private String contractId;
    private List<Argument> arguments;

    /**
     * Constructor for the spawn action.
     * @param contractId The contract ID.
     * @param arguments The initial arguments for running the contract.
     */
    public Spawn(String contractId, List<Argument> arguments) {
        this.contractId = contractId;
        this.arguments = arguments;
    }

    public Spawn(ByzCoinProto.Spawn proto) {
        contractId = proto.getContractid();
        arguments = new ArrayList<>();
        for (ByzCoinProto.Argument a : proto.getArgsList()) {
            arguments.add(new Argument(a));
        }
    }

    /**
     * Getter for contract ID.
     * @return The contract ID.
     */
    public String getContractId() {
        return contractId;
    }

    /**
     * Getter for the arguments.
     * @return The arguments.
     */
    public List<Argument> getArguments() {
        return arguments;
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public ByzCoinProto.Spawn toProto() {
        ByzCoinProto.Spawn.Builder b = ByzCoinProto.Spawn.newBuilder();
        b.setContractid(this.contractId);
        for (Argument a : this.arguments) {
            b.addArgs(a.toProto());
        }
        return b.build();
    }
}
