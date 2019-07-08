package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.darc.Identity;
import ch.epfl.dedis.lib.proto.ByzCoinProto;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;

/**
 * This is an improvement of the Instruction class to fix the hash function by including
 * the invoke command inside it.
 */
public class InstructionV1 extends Instruction {
    /**
     * Create an instruction from the previous version
     * @param instr The instruction to upgrade
     */
    public InstructionV1(Instruction instr) {
        super(instr.toProto());
    }

    /**
     * Create an instruction from the previous version
     * @param proto The protobuf-encoded instruction to upgrade
     */
    public InstructionV1(ByzCoinProto.Instruction proto) {
        super(proto);
    }

    @Override
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
                digest.update(this.invoke.getCommand().getBytes());
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
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }
}
