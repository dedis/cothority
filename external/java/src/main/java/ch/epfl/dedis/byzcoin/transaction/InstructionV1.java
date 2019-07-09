package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

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
        // Version 1 now includes the invoke command argument in the hash.
        return hash(1);
    }
}
