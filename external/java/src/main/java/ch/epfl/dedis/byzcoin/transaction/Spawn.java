package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

import java.util.ArrayList;
import java.util.List;

/**
 * Spawn is an operation that an Instruction can take, it should be used for creating an object.
 */
public class Spawn {
    private String contractID;
    private List<Argument> arguments;

    /**
     * Constructor for the spawn action.
     * @param contractID The contract ID.
     * @param arguments The initial arguments for running the contract.
     */
    public Spawn(String contractID, List<Argument> arguments) {
        this.contractID = contractID;
        this.arguments = arguments;
    }

    public Spawn(ByzCoinProto.Spawn proto) {
        contractID = proto.getContractid();
        arguments = new ArrayList<>();
        for (ByzCoinProto.Argument a : proto.getArgsList()) {
            arguments.add(new Argument(a));
        }
    }

    /**
     * Getter for contract ID.
     * @return The contract ID.
     */
    public String getContractID() {
        return contractID;
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
        b.setContractid(this.contractID);
        for (Argument a : this.arguments) {
            b.addArgs(a.toProto());
        }
        return b.build();
    }

    @Override
    public String toString() {
        return "contractID: " + this.contractID + ", argument: <hidden>";
    }
}
