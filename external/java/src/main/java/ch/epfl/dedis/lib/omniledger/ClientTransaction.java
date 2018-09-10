package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.OmniLedgerProto;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
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

    public ClientTransaction(OmniLedgerProto.ClientTransaction proto) {
        instructions = new ArrayList<Instruction>();
        for (OmniLedgerProto.Instruction i : proto.getInstructionsList()) {
            instructions.add(new Instruction(i));
        }
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
    public OmniLedgerProto.ClientTransaction toProto() {
        OmniLedgerProto.ClientTransaction.Builder b = OmniLedgerProto.ClientTransaction.newBuilder();
        for (Instruction instr : this.instructions) {
            b.addInstructions(instr.toProto());
        }
        return b.build();
    }

    public ClientTransactionId getId() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            for (Instruction instr : this.instructions) {
                digest.update(instr.hash());
            }
            return new ClientTransactionId(digest.digest());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        } catch (CothorityCryptoException e) {
            throw new RuntimeException(e);
        }
    }
}
