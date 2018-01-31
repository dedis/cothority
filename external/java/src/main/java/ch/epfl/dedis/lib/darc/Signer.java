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
     * @param msg
     * @return
     */
    byte[] sign(byte[] msg);

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return
     */
    Scalar getPrivate() throws CothorityCryptoException;

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return
     */
    Point getPublic() throws CothorityCryptoException;

    /**
     * Returns an identity of the signer.
     *
     * @return
     */
    Identity getIdentity() throws CothorityCryptoException;

    /**
     * Returns an array of bytes representing the signer. The first byte must indicate the type
     *
     * @return
     */
    byte[] serialize() throws IOException;

    boolean equals(Object o);
}
