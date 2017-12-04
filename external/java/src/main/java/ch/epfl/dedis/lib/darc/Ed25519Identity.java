package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.SchnorrSig;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

public class Ed25519Identity implements Identity {
    private Point pub;

    /**
     * Creates an Ed25519Identity from a protobuf representation.
     * @param proto
     */
    public Ed25519Identity(DarcProto.IdentityEd25519 proto){
        pub = new Point(proto.getPoint());
    }

    /**
     * Creates an Ed25519Identity from a Ed25519Signer.
     * @param signer
     */
    public Ed25519Identity(Signer signer) throws CothorityCryptoException{
        if (Ed25519Signer.class.isInstance(signer)) {
            pub = new Point(signer.getPublic());
        } else {
            throw new CothorityCryptoException("Wrong signer type: " + signer.toString());
        }
    }

    /**
     * Returns true if the verification of signature on the sha-256 of msg is
     * successful or false if not.
     * @param msg
     * @param signature
     * @return
     */
    public boolean verify(byte[] msg, byte[] signature){
        return new SchnorrSig(signature).verify(msg, pub);
    }

    /**
     * Creates a protobuf-representation of the implementation. The protobuf
     * representation has to hold all necessary fields to represent any of the
     * identity implementations.
     * @return
     */
    public DarcProto.Identity toProto(){
        DarcProto.Identity.Builder bid = DarcProto.Identity.newBuilder();
        DarcProto.IdentityEd25519.Builder bed = DarcProto.IdentityEd25519.newBuilder();
        bed.setPoint(ByteString.copyFrom(pub.toBytes()));
        bid.setEd25519(bed);
        return bid.build();
    }

    public String toString(){
        return pub.toString();
    }

    @Override
    public boolean equals(Object other) {
        if (other == null) return false;
        if (other == this) return true;
        if (!(other instanceof Ed25519Identity))return false;
        Ed25519Identity otherEd = (Ed25519Identity) other;
        return pub.equals(otherEd.pub);
    }
}
