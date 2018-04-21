package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.KeyPair;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Ed25519Scalar;
import ch.epfl.dedis.lib.crypto.SchnorrSig;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;

public class SignerEd25519 implements Signer {
    private Point pub;
    private Ed25519Scalar priv;

    private final Logger logger = LoggerFactory.getLogger(SignerEd25519.class);

    /**
     * Constructor for signer.
     */
    public SignerEd25519() {
        KeyPair kp = new KeyPair();
        pub = kp.Point;
        priv = kp.Ed25519Scalar;
    }

    /**
     * Creates a new signer from a slice of bytes. This must correspond to
     * what Ed25519.Ed25519Scalar.toBytes() returns.
     * @param data
     */
    public SignerEd25519(byte[] data){
        priv = new Ed25519Scalar(data);
        pub = priv.scalarMult(null);
    }

    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     *
     * @param msg
     * @return
     */
    public byte[] sign(byte[] msg) {
        SchnorrSig sig = new SchnorrSig(msg, priv);
        return sig.toBytes();
    }

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return
     */
    public Ed25519Scalar getPrivate() {
        return priv;
    }

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return
     */
    public Point getPublic() {
        return pub;
    }

    /**
     * Creates an identity of the signer.
     *
     * @return an identity
     * @throws CothorityCryptoException
     */
    public Identity getIdentity() throws CothorityCryptoException{
        return IdentityFactory.New(this);
    }

    /**
     * Returns an array of bytes representing the signer. The first byte must indicate the type
     *
     * @return
     */
    public byte[] serialize() throws IOException{
        byte[] result = new byte[1 + priv.toBytes().length];
        result[0] = SignerFactory.IDEd25519;
        System.arraycopy(priv.toBytes(), 0, result, 1, priv.toBytes().length);
        return result;
    }
}
