package ch.epfl.dedis.lib.byzcoin.darc;

import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

public class IdentityDarc implements Identity {
    private DarcId darcID;

    /**
     * Instantiates a IdentityDarc given its protobuf representation.
     *
     * @param proto
     */
    public IdentityDarc(DarcProto.IdentityDarc proto) throws CothorityCryptoException{
        darcID = new DarcId(proto.getId().toByteArray());
    }

    /**
     * Instantiates a IdentityDarc given a darc-id.
     *
     * @param darcID
     */
    public IdentityDarc(DarcId darcID) {
        this.darcID = darcID;
    }

    /**
     * Instantiates a IdentityDarc given a darc.
     * @param darc
     */
    public IdentityDarc(Darc darc)throws CothorityCryptoException{
        this(darc.getId());
    }

    /**
     * Returns true if the verification of signature on the sha-256 of msg is
     * successful or false if not.
     *
     * @param msg
     * @param signature
     * @return
     */
    public boolean verify(byte[] msg, byte[] signature) {
        return false;
    }

    /**
     * Creates a protobuf-representation of the implementation. The protobuf
     * representation has to hold all necessary fields to represent any of the
     * identity implementations.
     *
     * @return
     */
    public DarcProto.Identity toProto() {
        DarcProto.Identity.Builder bid = DarcProto.Identity.newBuilder();
        DarcProto.IdentityDarc.Builder bdd = DarcProto.IdentityDarc.newBuilder();
        bdd.setId(ByteString.copyFrom(darcID.getId()));
        bid.setDarc(bdd);
        return bid.build();
    }

    /**
     * Return ID of DARC
     *
     * @return ID of DARC
     */
    public DarcId getDarcId() {
        return darcID;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        IdentityDarc that = (IdentityDarc) o;

        return darcID != null ? darcID.equals(that.darcID) : that.darcID == null;
    }

    @Override
    public int hashCode() {
        return darcID != null ? darcID.hashCode() : 0;
    }

    public String toString() {
        return String.format("%s:%s", this.typeString(), Hex.printHexBinary(this.darcID.getId()).toLowerCase());
    }

    public String typeString() {
        return "darc";
    }
}
