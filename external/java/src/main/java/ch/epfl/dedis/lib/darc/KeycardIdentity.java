package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.SchnorrSig;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

/**
 * KeycardIdentity represents a keycard that holds its private key and can only be used to sign
 * but which will not reveal its private key.
 */
public class KeycardIdentity implements Identity {
    private byte[] pub;

    /**
     * Creates an KeycardIdentity from a protobuf representation.
     * @param proto
     */
    public KeycardIdentity(DarcProto.IdentityKeycard proto){
        pub = proto.getPublic().toByteArray();
    }

    /**
     * Creates an Ed25519Identity from a Ed25519Signer.
     * @param signer
     */
    public KeycardIdentity(Signer signer) throws CothorityCryptoException{
        if (KeycardSigner.class.isInstance(signer)) {
            pub = ((KeycardSigner) signer).publicBytes();
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
        // TODO: verify the signature given the msg, the signature and our public byte-array.
        return false;
    }

    /**
     * Creates a protobuf-representation of the implementation. The protobuf
     * representation has to hold all necessary fields to represent any of the
     * identity implementations.
     * @return
     */
    public DarcProto.Identity toProto(){
        DarcProto.Identity.Builder bid = DarcProto.Identity.newBuilder();
        DarcProto.IdentityKeycard.Builder bed = DarcProto.IdentityKeycard.newBuilder();
        bed.setPublic(ByteString.copyFrom(pub));
        bid.setKeycard(bed);
        return bid.build();
    }

    public String toString(){
        return pub.toString();
    }

    @Override
    public boolean equals(Object other) {
        if (other == null) return false;
        if (other == this) return true;
        if (!(other instanceof KeycardIdentity))return false;
        KeycardIdentity otherEd = (KeycardIdentity) other;
        return pub.equals(otherEd.pub);
    }
}
