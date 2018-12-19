package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.PointFactory;
import ch.epfl.dedis.lib.crypto.SchnorrSig;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.DarcProto;
import com.google.protobuf.ByteString;

public class IdentityEd25519 implements Identity {
    private Point pub;

    /**
     * Creates an IdentityEd25519 from a protobuf representation.
     * @param proto the protobuf to parse
     */
    public IdentityEd25519(DarcProto.IdentityEd25519 proto){
        pub = PointFactory.getInstance().fromProto(proto.getPoint());
    }

    /**
     * Creates an IdentityEd25519 from a Ed25519 point.
     * @param p Ed25519 point for the identity
     */
    public IdentityEd25519(Ed25519Point p){
        pub = p;
    }

    /**
     * Creates an IdentityEd25519 from a SignerEd25519.
     * @param signer the input signer
     */
    public IdentityEd25519(Signer signer) {
        if (signer instanceof SignerEd25519) {
            pub = signer.getPublic().copy();
        } else {
            throw new RuntimeException("Wrong signer type: " + signer.toString());
        }
    }

    /**
     * Returns true if the verification of signature on the sha-256 of msg is
     * successful or false if not.
     * @param msg the message
     * @param signature the signature
     * @return true if the signature is correct
     */
    public boolean verify(byte[] msg, byte[] signature) {
        try {
            return new SchnorrSig(signature).verify(msg, pub);
        } catch (CothorityCryptoException e) {
            return false;
        }
    }

    /**
     * Creates a protobuf-representation of the implementation. The protobuf
     * representation has to hold all necessary fields to represent any of the
     * identity implementations.
     * @return a protobuf-representation of the Identity
     */
    public DarcProto.Identity toProto(){
        DarcProto.Identity.Builder bid = DarcProto.Identity.newBuilder();
        DarcProto.IdentityEd25519.Builder bed = DarcProto.IdentityEd25519.newBuilder();
        bed.setPoint(pub.toProto());
        bid.setEd25519(bed);
        return bid.build();
    }

    public String toString(){
        return String.format("%s:%s", this.typeString(), this.pub.toString().toLowerCase());
    }

    public String typeString() {
        return "ed25519";
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
