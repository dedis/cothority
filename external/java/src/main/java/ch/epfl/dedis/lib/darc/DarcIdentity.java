package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

public class DarcIdentity implements Identity {
    private DarcId darcID;

    /**
     * Instantiates a DarcIdentity given its protobuf representation.
     *
     * @param proto
     */
    public DarcIdentity(DarcProto.IdentityDarc proto) throws CothorityCryptoException{
        darcID = new DarcId(proto.getId().toByteArray());
    }

    /**
     * Instantiates a DarcIdentity given a darc-id.
     *
     * @param darcID
     */
    public DarcIdentity(DarcId darcID) {
        this.darcID = darcID;
    }

    /**
     * Instantiates a DarcIdentity given a darc.
     * @param darc
     */
    public DarcIdentity(Darc darc)throws CothorityCryptoException{
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
}
