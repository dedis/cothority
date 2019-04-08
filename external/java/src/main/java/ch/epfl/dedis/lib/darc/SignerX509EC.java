package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.security.PublicKey;

/**
 * SignerX509EC represents a keycard that holds its private key and can only be used to sign
 * but which will not reveal its private key.
 * For the moment it _does_ hold its own private key, but in a later version this can be
 * removed and then any call to `sign` needs to go to the card.
 */
public abstract class SignerX509EC implements Signer {
    private final Logger logger = LoggerFactory.getLogger(SignerX509EC.class);
    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     *
     * @param msg the message
     * @return the signature
     */
    public abstract byte[] sign(byte[] msg) throws SignRequestRejectedException;

    /**
     * Returns the private key of the signer, or throws a CothorityCryptoException.
     *
     * @return the private key
     */
    public Scalar getPrivate()  {
        throw new RuntimeException("cannot reveal private key");
    }

    /**
     * Returns the public key of the signer, or throws a CothorityCryptoException.
     *
     * @return the public key
     */
    public Point getPublic()  {
        throw new RuntimeException("non-ed25519 public keys not yet implemented");
    }

    /**
     * Returns the identity of the signer.
     *
     * @return an identity
     */
    public Identity getIdentity() {
        return IdentityFactory.New(this);
    }

    /**
     * Returns an array of bytes repesenting the signer. The first byte must indicate the type.
     *
     * @return the serialized signer
     */
    public byte[] serialize() throws IOException {
        // TODO - serialize this thing so it can be recognized by go. The byte string must
        // start with a SignerFactory.Keycard byte, then comes whatever representation makes
        // most sense for these keycards.
        throw new IllegalStateException("It is not possible to serialise keycard signer as private key is in the card");
    }

    /**
     * Returns the specific public key representation of this signer
     * @return the output bytes
     *
     * TODO: implement something that makes sense here and that can be used by IdentityX509EC.
     *
     * bytes returned by this method are internal, binary representation of X509 key.
     * It should be possible to
     * <ol>
     *     <li>create a (@Link X509EncodedKeySpec key in following way: <pre>new X509EncodedKeySpec(SignerX509EC.publicBytes)</pre></li>
     *     <li>analyse returned value by openssl command</li>
     *     <li>use openssl to verify signature</li>
     * </ol>
     * EncodedKeySpec
     */
    public byte[] publicBytes() {
        return getPublicKey().getEncoded();
    }

    /**
     * Return public key as a class
     * @return the PublicKey
     */
    public abstract PublicKey getPublicKey();
}
