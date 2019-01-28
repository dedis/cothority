package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import ch.epfl.dedis.lib.proto.DarcProto;
import com.google.protobuf.ByteString;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;

/**
 * An instruction is sent and executed by ByzCoin.
 */
public class Instruction {
    private InstanceId instId;
    private Spawn spawn;
    private Invoke invoke;
    private Delete delete;
    private List<Long> signerCounters;
    private List<Signature> signatures;

    /**
     * Use this constructor if it is a spawn instruction, i.e. you want to create a new object.
     *
     * @param instId The instance ID.
     * @param ctrs   The list of monotonically increasing counter for those that will eventually sign the instruction.
     * @param spawn  The spawn object, which contains the value and the argument.
     */
    public Instruction(InstanceId instId, List<Long> ctrs, Spawn spawn) {
        this.instId = instId;
        this.signerCounters = ctrs;
        this.spawn = spawn;
    }

    /**
     * Use this constructor if it is an invoke instruction, i.e. you want to mutate an object.
     *
     * @param instId The ID of the object, which must be unique.
     * @param ctrs   The list of monotonically increasing counter for those that will eventually sign the instruction.
     * @param invoke The invoke object.
     */
    public Instruction(InstanceId instId, List<Long> ctrs, Invoke invoke) {
        this.instId = instId;
        this.signerCounters = ctrs;
        this.invoke = invoke;
    }

    /**
     * Use this constructor if it is a delete instruction, i.e. you want to delete an object.
     *
     * @param instId The ID of the object, which must be unique.
     * @param ctrs   The list of monotonically increasing counter for those that will eventually sign the instruction.
     * @param delete The delete object.
     */
    public Instruction(InstanceId instId, List<Long> ctrs, Delete delete) {
        this.instId = instId;
        this.signerCounters = ctrs;
        this.delete = delete;
    }

    public Instruction(ByzCoinProto.Instruction inst) throws CothorityCryptoException {
        this.instId = new InstanceId(inst.getInstanceid());
        if (inst.hasSpawn()) {
            this.spawn = new Spawn(inst.getSpawn());
        }
        if (inst.hasInvoke()) {
            this.invoke = new Invoke(inst.getInvoke());
        }
        if (inst.hasDelete()) {
            this.delete = new Delete(inst.getDelete());
        }
        this.signerCounters = new ArrayList<>();
        this.signerCounters.addAll(inst.getSignercounterList());
        this.signatures = new ArrayList<>();
        for (DarcProto.Signature sig : inst.getSignaturesList()) {
            this.signatures.add(new Signature(sig));
        }
    }

    /**
     * Getter for the instance ID.
     *
     * @return the InstanceID
     */
    public InstanceId getInstId() {
        return instId;
    }

    /**
     * Setter for the signer counters, they must map to the signers in the signature.
     *
     * @param signerCounters the list of counters
     */
    public void setSignerCounters(List<Long> signerCounters) {
        this.signerCounters = signerCounters;
    }

    /**
     * Setter for the signatures.
     *
     * @param signatures the signatures to set
     */
    public void setSignatures(List<Signature> signatures) {
        this.signatures = signatures;
    }

    /**
     * This method computes the sha256 hash of the instruction.
     *
     * @return The digest.
     */
    public byte[] hash() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(this.instId.getId());
            List<Argument> args = new ArrayList<>();
            if (this.spawn != null) {
                digest.update((byte) (0));
                digest.update(this.spawn.getContractId().getBytes());
                args = this.spawn.getArguments();
            } else if (this.invoke != null) {
                digest.update((byte) (1));
                digest.update(this.invoke.getContractId().getBytes());
                args = this.invoke.getArguments();
            } else if (this.delete != null) {
                digest.update((byte) (2));
                digest.update(this.delete.getContractId().getBytes());
            }
            for (Argument a : args) {
                digest.update(a.getName().getBytes());
                digest.update(a.getValue());
            }
            for (Long ctr : this.signerCounters) {
                ByteBuffer buffer = ByteBuffer.allocate(Long.BYTES);
                buffer.order(ByteOrder.LITTLE_ENDIAN);
                buffer.putLong(ctr);
                digest.update(buffer.array());
            }
            return digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * Converts this object to the protobuf representation.
     *
     * @return The protobuf representation.
     */
    public ByzCoinProto.Instruction toProto() {
        ByzCoinProto.Instruction.Builder b = ByzCoinProto.Instruction.newBuilder();
        b.setInstanceid(ByteString.copyFrom(this.instId.getId()));
        if (this.spawn != null) {
            b.setSpawn(this.spawn.toProto());
        } else if (this.invoke != null) {
            b.setInvoke(this.invoke.toProto());
        } else if (this.delete != null) {
            b.setDelete(this.delete.toProto());
        }
        b.addAllSignercounter(this.signerCounters);
        for (Signature s : this.signatures) {
            b.addSignatures(s.toProto());
        }
        return b.build();
    }

    /**
     * Outputs the action of the instruction, this action be the same as an action in the corresponding darc. Otherwise
     * this instruction may not be accepted.
     *
     * @return The action.
     */
    public String action() {
        String a = "invalid";
        if (this.spawn != null) {
            a = "spawn:" + this.spawn.getContractId();
        } else if (this.invoke != null) {
            a = "invoke:" + this.invoke.getContractId() + "." + this.invoke.getCommand();
        } else if (this.delete != null) {
            a = "delete:" + this.delete.getContractId();
        }
        return a;
    }

    /**
     * Have a list of signers sign the instruction. The instruction will *not* be accepted by byzcoin if it is not
     * signed. The signature will not be valid if the instruction is modified after signing.
     *
     * @param ctxHash - the hash of all instruction in the client transaction
     * @param signers - the list of signers.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public void signWith(byte[] ctxHash, List<Signer> signers) throws CothorityCryptoException {
        this.signatures = new ArrayList<>();
        for (Signer signer : signers) {
            try {
                this.signatures.add(new Signature(signer.sign(ctxHash), signer.getIdentity()));
            } catch (Signer.SignRequestRejectedException e) {
                throw new CothorityCryptoException(e.getMessage());
            }
        }
    }

    /**
     * This function derives a contract ID from the instruction with the given string. The derived instance ID if
     * useful as a key to this instruction.
     *
     * @param what - the string that gets mixed into the derived instance ID
     * @return the instance ID
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public InstanceId deriveId(String what) throws CothorityCryptoException {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(this.hash());
            digest.update(intToArr4(this.signatures.size()));
            for (Signature sig : this.signatures) {
                digest.update(intToArr4(sig.signature.length));
                digest.update(sig.signature);
            }
            digest.update(what.getBytes());
            return new InstanceId(digest.digest());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * @return the spawn-argument of the instruction - only one of spawn, invoke, and delete should be present.
     */
    public Spawn getSpawn() {
        return spawn;
    }

    /**
     * @return the invoke-argument of the instruction - only one of spawn, invoke, and delete should be present.
     */
    public Invoke getInvoke() {
        return invoke;
    }

    /**
     * @return the delete-argument of the instruction - only one of spawn, invoke, and delete should be present.
     */
    public Delete getDelete() {
        return delete;
    }

    /**
     * @return the signatures in this instruction.
     */
    public List<Signature> getSignatures() {
        return signatures;
    }

    private static byte[] intToArr4(int x) {
        ByteBuffer b = ByteBuffer.allocate(4);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putInt(x);
        return b.array();
    }
}
