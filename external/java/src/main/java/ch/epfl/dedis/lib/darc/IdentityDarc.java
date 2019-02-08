package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.proto.DarcProto;
import com.google.protobuf.ByteString;

public class IdentityDarc implements Identity {
    private DarcId darcBaseID;

    /**
     * Instantiates a IdentityDarc given its protobuf representation.
     *
     * @param proto the protobuf representation of the IdentityDarc
     */
    public IdentityDarc(DarcProto.IdentityDarc proto) {
        darcBaseID = new DarcId(proto.getId().toByteArray());
    }

    /**
     * Instantiates a IdentityDarc given a darc-id.
     *
     * @param darcBaseID the source DarcId
     */
    public IdentityDarc(DarcId darcBaseID) {
        this.darcBaseID = darcBaseID;
    }

    /**
     * Instantiates a IdentityDarc given a darc.
     * @param darc the source Darc
     */
    public IdentityDarc(Darc darc){
        this(darc.getId());
    }

    /**
     * Returns true if the verification of signature on the sha-256 of msg is
     * successful or false if not.
     *
     * @param msg the message
     * @param signature the signature
     * @return true if the signature is correct
     */
    public boolean verify(byte[] msg, byte[] signature) {
        return false;
    }

    /**
     * Creates a protobuf-representation of the implementation. The protobuf
     * representation has to hold all necessary fields to represent any of the
     * identity implementations.
     *
     * @return a protobuf-representation of the IdentityDarc
     */
    public DarcProto.Identity toProto() {
        DarcProto.Identity.Builder bid = DarcProto.Identity.newBuilder();
        DarcProto.IdentityDarc.Builder bdd = DarcProto.IdentityDarc.newBuilder();
        bdd.setId(ByteString.copyFrom(darcBaseID.getId()));
        bid.setDarc(bdd);
        return bid.build();
    }

    /**
     * Return ID of DARC
     *
     * @return ID of DARC
     */
    public DarcId getDarcId() {
        return darcBaseID;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        IdentityDarc that = (IdentityDarc) o;

        return darcBaseID != null ? darcBaseID.equals(that.darcBaseID) : that.darcBaseID == null;
    }

    @Override
    public int hashCode() {
        return darcBaseID != null ? darcBaseID.hashCode() : 0;
    }

    public String toString() {
        return String.format("%s:%s", this.typeString(), Hex.printHexBinary(this.darcBaseID.getId()).toLowerCase());
    }

    public String typeString() {
        return "darc";
    }

    public byte[] getPublicBytes() {
        return darcBaseID.getId();
    }
}
