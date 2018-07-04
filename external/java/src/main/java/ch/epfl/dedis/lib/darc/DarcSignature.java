package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcOCSProto;
import com.google.protobuf.ByteString;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;

public class DarcSignature {
    private byte[] signature;
    private SignaturePath path;

    private final Logger logger = LoggerFactory.getLogger(DarcSignature.class);

    /**
     * This will return a new DarcSignature by the signer on the message.
     * It will include all paths of the signer present in the signature.
     * The signed message is the sha-256 of the path concatenated with the msg:
     * sha256( path.getPathMsg() + msg )
     *
     * @param msg
     * @param path
     * @param signer
     */
    public DarcSignature(byte[] msg, SignaturePath path, Signer signer) throws CothorityCryptoException {
        this.path = path;
        try {
            signature = signer.sign(getHash(msg));
        } catch (Signer.SignRequestRejectedException e) {
            throw new CothorityCryptoException("Ugly user does not sign a request ", e);
        }
    }

    public DarcSignature(byte[] msg, Darc darc, Signer signer, int role) throws CothorityCryptoException {
        path = new SignaturePath(darc, signer, role);
        try {
            signature = signer.sign(getHash(msg));
        } catch (Signer.SignRequestRejectedException e) {
            throw new CothorityCryptoException("Ugly user does not sign a request ", e);
        }
    }

    /**
     * Recreates a darc from a protobuf representation.
     */
    public DarcSignature(DarcOCSProto.Signature proto) throws CothorityCryptoException{
        signature = proto.getSignature().toByteArray();
        path = new SignaturePath(proto.getSignaturepath());
    }

    /**
     * Returns the stored path in the signature.
     *
     * @return
     */
    public SignaturePath getPath() {
        return path;
    }

    /**
     * Returns true if the signature verification is OK, false on an error.
     * The base argument is the starting darc that is known to the document
     * or the skipchain-configuration.
     *
     * @param msg
     * @param base
     * @return
     */
    public boolean verify(byte[] msg, Darc base) throws CothorityCryptoException {
        if (path.getPathIDs().size() == 0){
            throw new CothorityCryptoException("cannot verify if path is not set - is this an online signature?");
        }
        if (!path.getPathIDs().get(0).equals(base.getId())) {
            return false;
        }
        return path.getSigner().verify(getHash(msg), signature);
    }

    /**
     * Returns the protobuf representation of the signature.
     *
     * @return
     */
    public DarcOCSProto.Signature toProto() {
        DarcOCSProto.Signature.Builder b = DarcOCSProto.Signature.newBuilder();
        b.setSignature(ByteString.copyFrom(signature));
        b.setSignaturepath(path.toProto());
        return b.build();
    }

    private byte[] getHash(byte[] msg) throws CothorityCryptoException {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            logger.debug("path: " + Hex.printHexBinary(path.getPathMsg()));
            logger.debug("msg: " + Hex.printHexBinary(msg));
            digest.update(path.getPathMsg());
            digest.update(msg);
            return digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new CothorityCryptoException("couldn't make hash: " + e.toString());
        }
    }

    @Override
    public boolean equals(Object other) {
        if (other == null) return false;
        if (other == this) return true;
        if (!(other instanceof DarcSignature)) return false;
        DarcSignature otherSig = (DarcSignature) other;
        return Arrays.equals(otherSig.signature, signature) &&
                otherSig.path.equals(path);
    }
}
