package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

public class DarcIdentity implements Identity {
    private byte[] darcID;

    /**
     * Instantiates a DarcIdentity given its protobuf representation.
     *
     * @param proto
     */
    public DarcIdentity(DarcProto.IdentityDarc proto) {
        darcID = proto.getId().toByteArray();
    }

    /**
     * Instantiates a DarcIdentity given a darc-id.
     *
     * @param darcID
     */
    public DarcIdentity(byte[] darcID) {
        this.darcID = darcID;
    }

    /**
     * Instantiates a DarcIdentity given a darc.
     * @param darc
     */
    public DarcIdentity(Darc darc){
        this(darc.ID());
    }

    /**
     * Returns true if the verification of signature on the sha-256 of msg is
     * successful or false if not.
     *
     * @param msg
     * @param signature
     * @return
     */
    public boolean Verify(byte[] msg, byte[] signature) {
        return false;
    }

    /**
     * Creates a protobuf-representation of the implementation. The protobuf
     * representation has to hold all necessary fields to represent any of the
     * identity implementations.
     *
     * @return
     */
    public DarcProto.Identity ToProto() {
        DarcProto.Identity.Builder bid = DarcProto.Identity.newBuilder();
        DarcProto.IdentityDarc.Builder bdd = DarcProto.IdentityDarc.newBuilder();
        bdd.setId(ByteString.copyFrom(darcID));
        bid.setDarc(bdd);
        return bid.build();
    }
}
