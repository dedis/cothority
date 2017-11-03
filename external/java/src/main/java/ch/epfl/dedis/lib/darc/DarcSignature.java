package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.xml.bind.DatatypeConverter;
import java.io.IOException;
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
     * sha256( path.GetPathMsg() + msg )
     *
     * @param msg
     * @param path
     * @param signer
     */
    public DarcSignature(byte[] msg, SignaturePath path, Signer signer) throws Exception {
        this.path = path;
        signature = signer.Sign(getHash(msg));
    }

    public DarcSignature(byte[] msg, Darc darc, Signer signer, int role) throws Exception {
        path = new SignaturePath(darc, signer, role);
        signature = signer.Sign(getHash(msg));
    }

    /**
     * Recreates a darc from a protobuf representation.
     */
    public DarcSignature(DarcProto.Signature proto) throws Exception {
        signature = proto.getSignature().toByteArray();
        path = new SignaturePath(proto.getSignaturepath());
    }

    /**
     * Returns the stored path in the signature.
     *
     * @return
     */
    public SignaturePath GetPath() {
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
    public boolean Verify(byte[] msg, Darc base) throws Exception {
        if (!Arrays.equals(path.GetPathIDs().get(0), base.ID())) {
            return false;
        }
        return path.GetSigner().Verify(getHash(msg), signature);
    }

    /**
     * Returns the protobuf representation of the signature.
     *
     * @return
     */
    public DarcProto.Signature ToProto() {
        DarcProto.Signature.Builder b = DarcProto.Signature.newBuilder();
        b.setSignature(ByteString.copyFrom(signature));
        b.setSignaturepath(path.ToProto());
        return b.build();
    }

    private byte[] getHash(byte[] msg) throws NoSuchAlgorithmException, IOException {
        MessageDigest digest = MessageDigest.getInstance("SHA-256");
        digest.update(path.GetPathMsg());
        digest.update(msg);
        return digest.digest();
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
