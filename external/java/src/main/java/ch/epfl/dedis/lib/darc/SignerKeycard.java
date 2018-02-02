package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.security.PublicKey;

/**
 * SignerKeycard represents a keycard that holds its private key and can only be used to sign
 * but which will not reveal its private key.
 * For the moment it _does_ hold its own private key, but in a later version this can be
 * removed and then any call to `sign` needs to go to the card.
 */
public abstract class SignerKeycard implements Signer {
    private final Logger logger = LoggerFactory.getLogger(SignerKeycard.class);
    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     *
     * @param msg
     * @return
     */
    public abstract byte[] sign(byte[] msg) throws SignRequestRejectedException;

    /**
     * Returns the private key of the signer, or throws a CothorityCryptoException.
     *
     * @return
     */
    public Scalar getPrivate() throws CothorityCryptoException {
        throw new CothorityCryptoException("cannot reveal private key");
    }

    /**
     * Returns the public key of the signer, or throws a CothorityCryptoException.
     *
     * @return
     */
    public Point getPublic() throws CothorityCryptoException {
        throw new CothorityCryptoException("non-ed25519 public keys not yet implemented");
    }

    /**
     * Creates an identity of the signer.
     *
     * @return an identity
     * @throws CothorityCryptoException
     */
    public Identity getIdentity() throws CothorityCryptoException {
        return IdentityFactory.New(this);
    }

    /**
     * Returns an array of bytes representing the signer. The first byte must indicate the type
     *
     * @return
     */
    public byte[] serialize() throws IOException {
        // TODO - serialize this thing so it can be recognized by go. The byte string must
        // start with a SignerFactory.Keycard byte, then comes whatever representation makes
        // most sense for these keycards.
        throw new IllegalStateException("It is not possible to serialise keycard signer as private key is in the card");
    }

    /**
     * Returns the specific public key representation of this signer
     * TODO: implement something that makes sense here and that can be used by
     * IdentityKeycard.
     *
     * bytes returned by this method are internal, binary representation of X509 key.
     * It should be possible to
     * <ol>
     *     <li>create a (@Link X509EncodedKeySpec key in following way: <pre>new X509EncodedKeySpec(SignerKeycard.publicBytes)</pre></li>
     *     <li>analyse returned value by openssl command</li>
     *     <li>use openssl to verify signature</li>
     * </ol>
     * EncodedKeySpec
     */
    public abstract byte[] publicBytes();

    /**
     * Return public key as a class
     * @return
     */
    public abstract PublicKey getPublicKey();
}
