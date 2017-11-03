package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;

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
    byte[] Sign(byte[] msg);

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return
     */
    Scalar GetPrivate();

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return
     */
    Point GetPublic();

    /**
     * Returns an array of bytes representing the signer. The first byte must indicate the type
     *
     * @return
     */
    byte[] Serialize() throws IOException;

    boolean equals(Object o);
}
