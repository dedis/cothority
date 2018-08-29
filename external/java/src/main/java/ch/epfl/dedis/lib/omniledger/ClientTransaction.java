package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.Sha256id;
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

    public ClientTransaction(OmniLedgerProto.ClientTransaction ct) throws CothorityCryptoException{
        instructions = new ArrayList<>();
        for (OmniLedgerProto.Instruction inst: ct.getInstructionsList()){
            instructions.add(new Instruction(inst));
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

    /**
     * @return the hash of the clientTransaction
     */
    public Sha256id hash(){
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            for (Instruction inst: instructions){
                digest.update(inst.hash());
            }
            return new Sha256id(digest.digest());
        } catch (NoSuchAlgorithmException | CothorityCryptoException e) {
            throw new RuntimeException(e);
        }
    }
}
