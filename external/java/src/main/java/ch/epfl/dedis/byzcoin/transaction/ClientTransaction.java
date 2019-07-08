package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.byzcoin.Block;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.darc.Identity;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

/**
 * ClientTransaction is a set of instructions are will be executed atomically by ByzCoin.
 */
public class ClientTransaction {
    private List<Instruction> instructions;

    /**
     * Constructor for the client transaction.
     * @param instructions The list of instructions that should be executed atomically.
     * @deprecated This function instantiates a deprecated list of instructions for latest
     * versions of the byzcoin protocol.
     */
    public ClientTransaction(List<Instruction> instructions) {
        this.instructions = instructions;
    }

    /**
     * Constructor for the client transaction.
     * @param instructions  The list of instructions that should be executed atomically.
     * @param version       Version of the ByzCoin protocol usually stored in the block header.
     */
    public ClientTransaction(List<Instruction> instructions, int version) {
        if (version >= 1) {
            this.instructions = instructions.stream().map(InstructionV1::new).collect(Collectors.toList());
        } else {
            this.instructions = instructions;
        }
    }

    /**
     *
     * @param proto Protobuf object
     * @deprecated This function instantiates a deprecated list of instructions for latest
     * versions of the byzcoin protocol.
     */
    public ClientTransaction(ByzCoinProto.ClientTransaction proto) {
        this(proto, 0);
    }

    /**
     * Constructor for the client transaction from a received message.
     * @param proto     Protobuf object
     * @param version   Version of the ByzCoin protocol usually stored in the block header.
     */
    public ClientTransaction(ByzCoinProto.ClientTransaction proto, int version) {
        instructions = new ArrayList<>();
        for (ByzCoinProto.Instruction i : proto.getInstructionsList()) {
            if (version >= 1) {
                instructions.add(new InstructionV1(i));
            } else {
                instructions.add(new Instruction(i));
            }
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
    public ByzCoinProto.ClientTransaction toProto() {
        ByzCoinProto.ClientTransaction.Builder b = ByzCoinProto.ClientTransaction.newBuilder();
        for (Instruction instr : this.instructions) {
            b.addInstructions(instr.toProto());
        }
        return b.build();
    }

    /**
     * This function signs all the instructions in the transaction using the same set of
     * signers. If some instructions need to be signed by different sets of
     * signers, then use the SighWith method from the Instruction class.
     * @param signers is the list of signers who signs all instructions
     */
    public void signWith(List<Signer> signers) throws CothorityCryptoException {
        List<Identity> ids = new ArrayList<>(signers.stream().map(Signer::getIdentity).collect(Collectors.toList()));
        for (Instruction instr : this.instructions) {
            instr.setSignerIdentities(ids);
        }
        byte[] h = this.hashInstructions();
        for (Instruction instr : this.instructions) {
            instr.signWith(h, signers);
        }
    }

    public ClientTransactionId getId() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            for (Instruction instr : this.instructions) {
                digest.update(instr.hash());
            }
            return new ClientTransactionId(digest.digest());
        } catch (NoSuchAlgorithmException | CothorityCryptoException e) {
            throw new RuntimeException(e);
        }
    }

    private byte[] hashInstructions() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            for (Instruction instr : this.instructions) {
                digest.update(instr.hash());
            }
            return digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

}
