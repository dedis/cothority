package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.TransactionProto;

import java.util.List;

public class ClientTransaction {
    private List<Instruction> instructions;

    public ClientTransaction(List<Instruction> instructions) {
        this.instructions = instructions;
    }

    public List<Instruction> getInstructions() {
        return instructions;
    }

    public TransactionProto.ClientTransaction toProto() {
        TransactionProto.ClientTransaction.Builder b = TransactionProto.ClientTransaction.newBuilder();
        for (Instruction instr : this.instructions) {
            b.addInstructions(instr.toProto());
        }
        return b.build();
    }
}
