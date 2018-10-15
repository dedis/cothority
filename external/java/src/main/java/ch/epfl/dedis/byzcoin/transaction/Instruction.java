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
import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.List;

/**
 * An instruction is sent and executed by ByzCoin.
 */
public class Instruction {
    private InstanceId instId;
    private byte[] nonce;
    private int index;
    private int length;
    private Spawn spawn;
    private Invoke invoke;
    private Delete delete;
    private List<Signature> signatures;

    /**
     * Use this constructor if it is a spawn instruction, i.e. you want to create a new object.
     *
     * @param instId The instance ID.
     * @param nonce  The nonce of the object.
     * @param index  The index of the instruction in the atomic set.
     * @param length The length of the atomic set.
     * @param spawn  The spawn object, which contains the value and the argument.
     */
    public Instruction(InstanceId instId, byte[] nonce, int index, int length, Spawn spawn) {
        this.instId = instId;
        this.nonce = nonce;
        this.index = index;
        this.length = length;
        this.spawn = spawn;
    }

    /**
     * Use this constructor if it is an invoke instruction, i.e. you want to mutate an object.
     *
     * @param instId The ID of the object, which must be unique.
     * @param nonce  The nonce of the object.
     * @param index  The index of the instruction in the atomic set.
     * @param length The length of the atomic set.
     * @param invoke The invoke object.
     */
    public Instruction(InstanceId instId, byte[] nonce, int index, int length, Invoke invoke) {
        this.instId = instId;
        this.nonce = nonce;
        this.index = index;
        this.length = length;
        this.invoke = invoke;
    }

    /**
     * Use this constructor if it is a delete instruction, i.e. you want to delete an object.
     *
     * @param instId The ID of the object, which must be unique.
     * @param nonce  The nonce of the object.
     * @param index  The index of the instruction in the atomic set.
     * @param length The length of the atomic set.
     * @param delete The delete object.
     */
    public Instruction(InstanceId instId, byte[] nonce, int index, int length, Delete delete) {
        this.instId = instId;
        this.nonce = nonce;
        this.index = index;
        this.length = length;
        this.delete = delete;
    }

    public Instruction(ByzCoinProto.Instruction inst) throws CothorityCryptoException {
        this.instId = new InstanceId(inst.getInstanceid());
        this.nonce = inst.getNonce().toByteArray();
        this.index = inst.getIndex();
        this.length = inst.getLength();
        if (inst.hasSpawn()) {
            this.spawn = new Spawn(inst.getSpawn());
        }
        if (inst.hasInvoke()) {
            this.invoke = new Invoke(inst.getInvoke());
        }
        if (inst.hasDelete()) {
            this.delete = new Delete(inst.getDelete());
        }
        this.signatures = new ArrayList<Signature>();
        for (DarcProto.Signature sig : inst.getSignaturesList()) {
            this.signatures.add(new Signature(sig));
        }
    }

    /**
     * Getter for the instance ID.
     * @return the InstanceID
     */
    public InstanceId getInstId() {
        return instId;
    }

    /**
     * Setter for the signatures.
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
            digest.update(this.nonce);
            digest.update(intToArr4(this.index));
            digest.update(intToArr4(this.length));
            List<Argument> args = new ArrayList<>();
            if (this.spawn != null) {
                digest.update((byte) (0));
                digest.update(this.spawn.getContractId().getBytes());
                args = this.spawn.getArguments();
            } else if (this.invoke != null) {
                digest.update((byte) (1));
                args = this.invoke.getArguments();
            } else if (this.delete != null) {
                digest.update((byte) (2));
            }
            for (Argument a : args) {
                digest.update(a.getName().getBytes());
                digest.update(a.getValue());
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
        b.setNonce(ByteString.copyFrom(this.nonce));
        b.setIndex(this.index);
        b.setLength(this.length);
        if (this.spawn != null) {
            b.setSpawn(this.spawn.toProto());
        } else if (this.invoke != null) {
            b.setInvoke(this.invoke.toProto());
        } else if (this.delete != null) {
            b.setDelete(this.delete.toProto());
        }
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
            a = "invoke:" + this.invoke.getCommand();
        } else if (this.delete != null) {
            a = "delete";
        }
        return a;
    }

    /**
     * Converts the instruction to a Darc request representation.
     *
     * @return The Darc request.
     * @param darcId the input darc ID
     */
    public Request toDarcRequest(DarcId darcId) {
        List<Identity> ids = new ArrayList<>();
        List<byte[]> sigs = new ArrayList<>();
        for (Signature sig : this.signatures) {
            ids.add(sig.signer);
            sigs.add(sig.signature);
        }
        return new Request(darcId, this.action(), this.hash(), ids, sigs);
    }

    /**
     * Have a list of signers sign the instruction. The instruction will *not* be accepted by byzcoin if it is not
     * signed. The signature will not be valid if the instruction is modified after signing.
     *
     * @param darcId - the darcId
     * @param signers - the list of signers.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public void signBy(DarcId darcId, List<Signer> signers) throws CothorityCryptoException {
        this.signatures = new ArrayList<>();
        for (Signer signer : signers) {
            this.signatures.add(new Signature(null, signer.getIdentity()));
        }

        byte[] msg = this.toDarcRequest(darcId).hash();
        for (int i = 0; i < this.signatures.size(); i++) {
            try {
                this.signatures.set(i, new Signature(signers.get(i).sign(msg), signers.get(i).getIdentity()));
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
     * @return the nonce of the instruction.
     */
    public byte[] getNonce() {
        return nonce;
    }

    /**
     * @return the index of the instruction - should be always smaller than the length.
     */
    public int getIndex() {
        return index;
    }

    /**
     * @return the length of the instruction - should be always bigger than the index.
     */
    public int getLength() {
        return length;
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

    /**
     * TODO: define how nonces are used.
     * @return generates a nonce to be used in the instructions.
     */
    public static byte[] genNonce()  {
        SecureRandom sr = new SecureRandom();
        byte[] nonce = new byte[32];
        sr.nextBytes(nonce);
        return nonce;
    }

    private static byte[] intToArr4(int x) {
        ByteBuffer b = ByteBuffer.allocate(4);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putInt(x);
        return b.array();
    }
}
