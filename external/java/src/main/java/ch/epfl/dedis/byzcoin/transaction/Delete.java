package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

/**
 * Delete is an operation that an Instruction can take, it should be used for deleting an object.
 */
public class Delete {
    private final String contractID;
    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public ByzCoinProto.Delete toProto() {
        ByzCoinProto.Delete.Builder b = ByzCoinProto.Delete.newBuilder();
        b.setContractid(contractID);
        return b.build();
    }

    /**
     * Constructor from protobuf.
     * @param proto the input protobuf
     */
    public Delete(ByzCoinProto.Delete proto) {
        this.contractID = proto.getContractid();
    }

    /**
     * Getter for the contract ID.
     */
    public String getContractId() {
        return contractID;
    }
}
