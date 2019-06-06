package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;

public class SignerEd25519 implements Signer {
    private Point pub;
    private Scalar priv;

    private final Logger logger = LoggerFactory.getLogger(SignerEd25519.class);

    /**
     * Constructor for signer.
     */
    public SignerEd25519() {
        Ed25519Pair kp = new Ed25519Pair();
        pub = kp.point;
        priv = kp.scalar;
    }

    /**
     * Creates a new signer from a slice of bytes. This must correspond to
     * what Ed25519.prime_order.toBytes() returns. This constructor does not
     * correctly decode the result of serialize(). Please use
     * SignerFactory.New instead.
     * @param data a public key in byte form
     */
    public SignerEd25519(byte[] data){
        priv = new Ed25519Scalar(data);
        pub = Ed25519Point.base().mul(priv);
    }

    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     *
     * @param msg the message
     * @return the signature
     */
    @Override
    public byte[] sign(byte[] msg) {
        SchnorrSig sig = new SchnorrSig(msg, priv);
        return sig.toBytes();
    }

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return the private key
     */
    @Override
    public Scalar getPrivate() {
        return priv;
    }

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return the public key
     */
    @Override
    public Point getPublic() {
        return pub;
    }

    /**
     * Creates an identity of the signer.
     *
     * @return an identity
     */
    @Override
    public Identity getIdentity() {
        return IdentityFactory.New(this);
    }

    /**
     * Returns an array of bytes representing the signer. The first byte must indicate the type.
     *
     * @return the serialised signer
     * @throws IOException if something went wrong with I/O
     */
    @Override
    public byte[] serialize() throws IOException{
        byte[] result = new byte[1 + priv.toBytes().length];
        result[0] = SignerFactory.IDEd25519;
        System.arraycopy(priv.toBytes(), 0, result, 1, priv.toBytes().length);
        return result;
    }
}
