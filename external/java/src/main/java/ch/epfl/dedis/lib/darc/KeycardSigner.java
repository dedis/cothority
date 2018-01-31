package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.KeyPair;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.crypto.SchnorrSig;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;

/**
 * KeycardSigner represents a keycard that holds its private key and can only be used to sign
 * but which will not reveal its private key.
 * For the moment it _does_ hold its own private key, but in a later version this can be
 * removed and then any call to `sign` needs to go to the card.
 */
public class KeycardSigner implements Signer {
    // TODO: chose correct type here
    private byte[] pub;

    private final Logger logger = LoggerFactory.getLogger(KeycardSigner.class);

    /**
     * Constructor for signer.
     */
    public KeycardSigner() {
        // TODO: create a new signer - perhaps doesn't make sense in the case of keycards.
        // for testing, creating a random signer might be nice anyway.
    }

    /**
     * Constructor for signer.
     */
    public KeycardSigner(byte[] buf) {
        // TODO: create a new signer from a known public key - for testing it might be nice
        // to actually use a private key here, too.
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
        // TODO: create correct signature
        return null;
    }

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
    public Point getPublic() throws CothorityCryptoException{
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
        return null;
    }

    /**
     * Returns the specific public key representation of this signer
     * TODO: implement something that makes sense here and that can be used by
     * KeycardIdentity.
     */
    public byte[] publicBytes(){
        return pub;
    }
}
