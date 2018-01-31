package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;

public class IdentityFactory {
    /**
     * Returns an instantiated identity that is stored in proto.
     */
    public static Identity New(DarcProto.Identity proto) throws CothorityCryptoException{
        if (proto.hasEd25519()) {
            return new Ed25519Identity(proto.getEd25519());
        } else if (proto.hasDarc()) {
            return new DarcIdentity(proto.getDarc());
        } else if (proto.hasKeycard()) {
            return new KeycardIdentity(proto.getKeycard());
        } else {
            throw new CothorityCryptoException("No identity present");
        }
    }

    /**
     * Creates the corresponding identity to a signer.
     *
     * @param signer
     */
    public static Identity New(Signer signer) throws CothorityCryptoException {
        if (Ed25519Signer.class.isInstance(signer)) {
            return new Ed25519Identity(signer);
        } else if (KeycardSigner.class.isInstance(signer)) {
            return new KeycardIdentity(signer);
        } else {
            throw new CothorityCryptoException("Cannot make Identity out of " + signer.toString());
        }
    }

    /**
     * Creates the corresponding identity to a darc.
     *
     * @param darc
     */
    public static Identity New(Darc darc) throws CothorityCryptoException {
        return new DarcIdentity(darc);
    }
}
