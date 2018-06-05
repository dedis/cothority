package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;

import java.util.List;

/**
 * ClientTransaction is a set of instructions are will be executed atomically by OmniLedger.
 */
public class ClientTransaction {
    private List<Instruction> instructions;

    /**
     * Constructor for the client transaction.
     * @param instructions The list of instruction that should be executed atomically.
     */
    public ClientTransaction(List<Instruction> instructions) {
        this.instructions = instructions;
    }

    /**
     * Getter for the instructions.
     * @return The instructions.
     */
    public List<Instruction> getInstructions() {
        return instructions;
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public TransactionProto.ClientTransaction toProto() {
        TransactionProto.ClientTransaction.Builder b = TransactionProto.ClientTransaction.newBuilder();
        for (Instruction instr : this.instructions) {
            b.addInstructions(instr.toProto());
        }
        return b.build();
    }
}
