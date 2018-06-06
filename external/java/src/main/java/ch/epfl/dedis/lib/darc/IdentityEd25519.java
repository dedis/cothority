package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.crypto.SchnorrSig;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcOCSProto;
import com.google.protobuf.ByteString;

public class IdentityEd25519 implements Identity {
    private Point pub;

    /**
     * Creates an IdentityEd25519 from a protobuf representation.
     * @param proto
     */
    public IdentityEd25519(DarcOCSProto.IdentityEd25519 proto){
        pub = new Ed25519Point(proto.getPoint());
    }

    /**
     * Creates an IdentityEd25519 from a SignerEd25519.
     * @param signer
     */
    public IdentityEd25519(Signer signer) throws CothorityCryptoException{
        if (SignerEd25519.class.isInstance(signer)) {
            pub = new Ed25519Point(signer.getPublic());
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
    public DarcOCSProto.Identity toProto(){
        DarcOCSProto.Identity.Builder bid = DarcOCSProto.Identity.newBuilder();
        DarcOCSProto.IdentityEd25519.Builder bed = DarcOCSProto.IdentityEd25519.newBuilder();
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
        if (!(other instanceof IdentityEd25519))return false;
        IdentityEd25519 otherEd = (IdentityEd25519) other;
        return pub.equals(otherEd.pub);
    }
}
