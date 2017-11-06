package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.KeyPair;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.crypto.SchnorrSig;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;

public class Ed25519Signer implements Signer {
    private Point pub;
    private Scalar priv;

    private final Logger logger = LoggerFactory.getLogger(Ed25519Signer.class);

    /**
     * Constructor for signer.
     */
    public Ed25519Signer() {
        KeyPair kp = new KeyPair();
        pub = kp.Point;
        priv = kp.Scalar;
    }

    /**
     * Creates a new signer from a slice of bytes. This must correspond to
     * what Ed25519.Scalar.toBytes() returns.
     * @param data
     */
    public Ed25519Signer(byte[] data){
        priv = new Scalar(data);
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
    public byte[] Sign(byte[] msg) {
        SchnorrSig sig = new SchnorrSig(msg, priv);
        return sig.toBytes();
    }

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return
     */
    public Scalar GetPrivate() {
        return priv;
    }

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return
     */
    public Point GetPublic() {
        return pub;
    }

    /**
     * Returns an array of bytes representing the signer. The first byte must indicate the type
     *
     * @return
     */
    public byte[] Serialize() throws IOException{
        byte[] result = new byte[1 + priv.toBytes().length];
        result[0] = SignerFactory.IDEd25519;
        System.arraycopy(priv.toBytes(), 0, result, 1, priv.toBytes().length);
        return result;
    }
}
