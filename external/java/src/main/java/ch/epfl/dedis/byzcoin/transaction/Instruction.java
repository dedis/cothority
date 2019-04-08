package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.darc.Identity;
import ch.epfl.dedis.lib.darc.IdentityFactory;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.ByteString;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

/**
 * An instruction is sent and executed by ByzCoin.
 */
public class Instruction {
    private InstanceId instId;
    private Spawn spawn;
    private Invoke invoke;
    private Delete delete;
    private List<Identity> signerIdentities;
    private List<Long> signerCounters;
    private List<byte[]> signatures;

    /**
     * Use this constructor if it is a spawn instruction, i.e. you want to create a new object.
     *
     * @param instId The instance ID.
     * @param ids    The identities of all the signers.
     * @param ctrs   The list of monotonically increasing counter for those that will eventually sign the instruction.
     * @param spawn  The spawn object, which contains the value and the argument.
     */
    public Instruction(InstanceId instId, List<Identity> ids, List<Long> ctrs, Spawn spawn) {
        this.instId = instId;
        this.signerIdentities = ids;
        this.signerCounters = ctrs;
        this.spawn = spawn;
    }

    /**
     * Use this constructor if it is an invoke instruction, i.e. you want to mutate an object.
     *
     * @param instId The ID of the object, which must be unique.
     * @param ids    The identities of all the signers.
     * @param ctrs   The list of monotonically increasing counter for those that will eventually sign the instruction.
     * @param invoke The invoke object.
     */
    public Instruction(InstanceId instId, List<Identity> ids, List<Long> ctrs, Invoke invoke) {
        this.instId = instId;
        this.signerIdentities = ids;
        this.signerCounters = ctrs;
        this.invoke = invoke;
    }

    /**
     * Use this constructor if it is a delete instruction, i.e. you want to delete an object.
     *
     * @param instId The ID of the object, which must be unique.
     * @param ids    The identities of all the signers.
     * @param ctrs   The list of monotonically increasing counter for those that will eventually sign the instruction.
     * @param delete The delete object.
     */
    public Instruction(InstanceId instId, List<Identity> ids, List<Long> ctrs, Delete delete) {
        this.instId = instId;
        this.signerCounters = ctrs;
        this.signerIdentities = ids;
        this.delete = delete;
    }

    /**
     * Construct Instruction from its protobuf representation.
     *
     * @param inst the protobuf representation of the instruction
     */
    public Instruction(ByzCoinProto.Instruction inst) {
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

        this.signerIdentities = new ArrayList<>();
        this.signerIdentities.addAll(inst.getSigneridentitiesList()
                .stream().map(IdentityFactory::New).collect(Collectors.toList()));

        this.signerCounters = new ArrayList<>();
        this.signerCounters.addAll(inst.getSignercounterList());

        this.signatures = new ArrayList<>();
        this.signatures.addAll(inst.getSignaturesList()
                .stream().map(x -> x.toByteArray()).collect(Collectors.toList()));
    }

    /**
     * Getter for signer identities.
     */
    public List<Identity> getSignerIdentities() {
        return signerIdentities;
    }

    /**
     * Getter for signer counters.
     */
    public List<Long> getSignerCounters() {
        return signerCounters;
    }

    /**
     * Getter for the signatures.
     *
     * @return the signatures in this instruction.
     */
    public List<byte[]> getSignatures() {
        return signatures;
    }

    /**
     * Getter for the instance ID.
     *
     * @return the InstanceID
     */
    public InstanceId getInstanceId() {
        return instId;
    }

    /**
     * Getter for the spawn argument.
     *
     * @return the spawn-argument of the instruction - only one of spawn, invoke, and delete should be present.
     */
    public Spawn getSpawn() {
        return spawn;
    }

    /**
     * Getter for the invoke argument.
     *
     * @return the invoke-argument of the instruction - only one of spawn, invoke, and delete should be present.
     */
    public Invoke getInvoke() {
        return invoke;
    }

    /**
     * Getter for the delete argument.
     *
     * @return the delete-argument of the instruction - only one of spawn, invoke, and delete should be present.
     */
    public Delete getDelete() {
        return delete;
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
    public void setSignatures(List<byte[]> signatures) {
        this.signatures = signatures;
    }

    public void setSignerIdentities(List<Identity> signerIdentities) {
        this.signerIdentities = signerIdentities;
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
                digest.update(this.spawn.getContractID().getBytes());
                args = this.spawn.getArguments();
            } else if (this.invoke != null) {
                digest.update((byte) (1));
                digest.update(this.invoke.getContractID().getBytes());
                args = this.invoke.getArguments();
            } else if (this.delete != null) {
                digest.update((byte) (2));
                digest.update(this.delete.getContractId().getBytes());
            }
            for (Argument a : args) {
                byte[] nameBuf = a.getName().getBytes();
                ByteBuffer nameLenBuf = ByteBuffer.allocate(Long.BYTES);
                nameLenBuf.order(ByteOrder.LITTLE_ENDIAN);
                nameLenBuf.putLong(nameBuf.length);
                digest.update(nameLenBuf.array());
                digest.update(nameBuf);

                ByteBuffer valueLenBuf = ByteBuffer.allocate(Long.BYTES);
                valueLenBuf.order(ByteOrder.LITTLE_ENDIAN);
                valueLenBuf.putLong(a.getValue().length);
                digest.update(valueLenBuf.array());
                digest.update(a.getValue());
            }
            for (Long ctr : this.signerCounters) {
                ByteBuffer ctrBuf = ByteBuffer.allocate(Long.BYTES);
                ctrBuf.order(ByteOrder.LITTLE_ENDIAN);
                ctrBuf.putLong(ctr);
                digest.update(ctrBuf.array());
            }
            for (Identity id : this.signerIdentities) {
                byte[] buf = id.getPublicBytes();
                ByteBuffer lenBuf = ByteBuffer.allocate(Long.BYTES);
                lenBuf.order(ByteOrder.LITTLE_ENDIAN);
                lenBuf.putLong(buf.length);
                digest.update(lenBuf.array());
                digest.update(buf);
            }
            return digest.digest();
        } catch (NoSuchAlgorithmException  e) {
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
        b.addAllSigneridentities(this.signerIdentities
                .stream().map(Identity::toProto).collect(Collectors.toList()));
        b.addAllSignercounter(this.signerCounters);
        b.addAllSignatures(this.signatures
                .stream().map(ByteString::copyFrom).collect(Collectors.toList()));
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
            a = "spawn:" + this.spawn.getContractID();
        } else if (this.invoke != null) {
            a = "invoke:" + this.invoke.getContractID() + "." + this.invoke.getCommand();
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
        if (signers.size() != this.signerIdentities.size()) {
            throw new CothorityCryptoException("the number of signers does not match the number of identities");
        }
        if (signers.size() != this.signerCounters.size()) {
            throw new CothorityCryptoException("the number of signers does not match the number of counters");
        }
        this.signatures = new ArrayList<>();
        for (int i = 0; i < signers.size(); i++) {
            Identity signerID = signers.get(i).getIdentity();
            if (!this.signerIdentities.get(i).equals(signerID)) {
                throw new CothorityCryptoException("signer identity is not set correctly");
            }
            try {
                this.signatures.add(signers.get(i).sign(ctxHash));
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
     */
    public InstanceId deriveId(String what) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(this.hash());
            digest.update(intToArr4(this.signatures.size()));
            for (byte[] sig : this.signatures) {
                digest.update(intToArr4(sig.length));
                digest.update(sig);
            }
            digest.update(what.getBytes());
            return new InstanceId(digest.digest());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public String toString() {
        StringBuffer out = new StringBuffer("");
        out.append(String.format("instr %s\n", Hex.printHexBinary(this.hash())));
        out.append(String.format("\taction: %s\n", this.action()));
        // TODO use proper strings
        out.append(String.format("\tidentities: %d\n", this.signerIdentities.size()));
        out.append(String.format("\tcounters: %d\n", this.signerCounters.size()));
        out.append(String.format("\tsignatures: %d\n", this.signatures.size()));
        return out.toString();
    }

    private static byte[] intToArr4(int x) {
        ByteBuffer b = ByteBuffer.allocate(4);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putInt(x);
        return b.array();
    }
}
