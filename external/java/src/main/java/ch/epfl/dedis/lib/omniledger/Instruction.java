package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.omniledger.darc.Identity;
import ch.epfl.dedis.lib.omniledger.darc.Request;
import ch.epfl.dedis.lib.omniledger.darc.Signature;
import ch.epfl.dedis.proto.TransactionProto;
import com.google.protobuf.ByteString;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;

/**
 * An instruction is sent and executed by OmniLedger.
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
     * @param instId The ID of the object, which must be unique.
     * @param nonce The nonce of the object.
     * @param index The index of the instruction in the atomic set.
     * @param length The length of the atomic set.
     * @param spawn The spawn object, which contains the value and the argument.
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
     * @param instId The ID of the object, which must be unique.
     * @param nonce The nonce of the object.
     * @param index The index of the instruction in the atomic set.
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
     * @param instId The ID of the object, which must be unique.
     * @param nonce The nonce of the object.
     * @param index The index of the instruction in the atomic set.
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

    /**
     * Getter for the object ID.
     */
    public InstanceId getInstId() {
        return instId;
    }

    /**
     * Setter for the signatures.
     */
    public void setSignatures(List<Signature> signatures) {
        this.signatures = signatures;
    }

    /**
     * This method computes the sha256 hash of the instruction.
     * @return The digest.
     */
    public byte[] hash() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(this.instId.getDarcId().getId());
            digest.update(this.instId.getSubId().getId());
            digest.update(this.nonce);
            digest.update(intToArr4(this.index));
            digest.update(intToArr4(this.length));
            List<Argument> args= new ArrayList<>();
            if (this.spawn != null) {
                digest.update((byte)(0));
                digest.update(this.spawn.getContractId().getBytes());
                args = this.spawn.getArguments();
            } else if (this.invoke != null) {
                digest.update((byte)(1));
                args = this.invoke.getArguments();
            } else if (this.delete != null) {
                digest.update((byte)(2));
            }
            for (Argument a : args) {
                digest.update(a.getName().getBytes());
                digest.update(a.getValue());
            }
            return digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        } catch (CothorityCryptoException e){
            throw new RuntimeException(e);
        }
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public TransactionProto.Instruction toProto() {
        TransactionProto.Instruction.Builder b = TransactionProto.Instruction.newBuilder();
        b.setObjectid(this.instId.toProto());
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
     * @return The action.
     */
    public String action() {
        String a = "invalid";
        if (this.spawn != null ) {
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
     * @return The Darc request.
     */
    public Request toDarcRequest() {
        List<Identity> ids = new ArrayList<>();
        List<byte[]> sigs = new ArrayList<>();
        for (Signature sig : this.signatures) {
            ids.add(sig.signer);
            sigs.add(sig.signature);
        }
        try {
            return new Request(this.instId.getDarcId(), this.action(), this.hash(), ids, sigs);
        } catch (CothorityCryptoException e){
            throw new RuntimeException(e);
        }
    }

    private static byte[] intToArr4(int x) {
        ByteBuffer b = ByteBuffer.allocate(4);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putInt(x);
        return b.array();
    }
}
