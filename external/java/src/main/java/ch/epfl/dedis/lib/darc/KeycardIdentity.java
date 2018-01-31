package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

import java.security.InvalidKeyException;
import java.security.KeyFactory;
import java.security.NoSuchAlgorithmException;
import java.security.PublicKey;
import java.security.Signature;
import java.security.SignatureException;
import java.security.spec.InvalidKeySpecException;
import java.security.spec.X509EncodedKeySpec;
import java.util.Arrays;

/**
 * KeycardIdentity represents a keycard that holds its private key and can only be used to sign
 * but which will not reveal its private key.
 */
public class KeycardIdentity implements Identity {
    private final PublicKey pubKey;

    /**
     * Creates an KeycardIdentity from a protobuf representation.
     * @param proto
     */
    public KeycardIdentity(DarcProto.IdentityKeycard proto) throws CothorityCryptoException {
        try {
            KeyFactory keyFactory = KeyFactory.getInstance("EC");
            X509EncodedKeySpec pubSpec = new X509EncodedKeySpec(proto.getPublic().toByteArray());
            pubKey = keyFactory.generatePublic(pubSpec);
        } catch (InvalidKeySpecException | NoSuchAlgorithmException e) {
            throw new CothorityCryptoException("Unable to deserialise KeycardIdentity identity", e);
        }
    }

    /**
     * Creates an Ed25519Identity from a Ed25519Signer.
     * @param signer
     */
    public KeycardIdentity(Signer signer) throws CothorityCryptoException{
        if (KeycardSigner.class.isInstance(signer)) {
            pubKey = ((KeycardSigner) signer).getPublicKey();
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
    public boolean verify(byte[] msg, byte[] signature) {
        // TODO: it is interesting why client code need to verify keycard singature ? It should
        // be verified at server side.
        // TODO: verify the signature given the msg, the signature and our public byte-array.
        try {
            final Signature signature2 = Signature.getInstance("SHA384withECDSA");
            signature2.initVerify(pubKey);
            signature2.update(msg);
            return signature2.verify(signature);
        } catch (InvalidKeyException | SignatureException | NoSuchAlgorithmException e) {
            // TODO: consider throwing some exception
            return false;
        }
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
        bed.setPublic(ByteString.copyFrom(pubKey.getEncoded()));
        bid.setKeycard(bed);
        return bid.build();
    }

    public String toString(){
        return pubKey.getEncoded().toString();
    }

    @Override
    public boolean equals(Object other) {
        if (other == null) return false;
        if (other == this) return true;
        if (!(other instanceof KeycardIdentity))return false;
        KeycardIdentity otherEd = (KeycardIdentity) other;
        return Arrays.equals(pubKey.getEncoded(), otherEd.pubKey.getEncoded());
    }
}
