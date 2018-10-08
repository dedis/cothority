package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

import java.io.IOException;

public interface Signer {
    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     *
     * @param msg a message
     * @return the signature
     * @throws SignRequestRejectedException if the signature request is rejected
     */
    byte[] sign(byte[] msg) throws SignRequestRejectedException;

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return the private key
     */
    Scalar getPrivate();

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return the public key
     */
    Point getPublic();

    /**
     * Returns an identity of the signer.
     *
     * @return the identity
     */
    Identity getIdentity();

    /**
     * Returns an array of bytes representing the signer. The first byte must indicate the type
     *
     * @return the serialized signer
     * @throws IOException if something went wrong with I/O
     */
    byte[] serialize() throws IOException;

    class SignRequestRejectedException extends Exception {
        public SignRequestRejectedException(String message, Throwable cause) {
            super(message, cause);
        }
        // TODO: this exception should be moved to a proper place
    }
}
