package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;

public class IdentityFactory {
    /**
     * Returns an instantiated identity that is stored in proto.
     */
    public static Identity New(DarcProto.Identity proto) throws CothorityCryptoException{
        if (proto.hasEd25519()) {
            return new IdentityEd25519(proto.getEd25519());
        } else if (proto.hasDarc()) {
            return new IdentityDarc(proto.getDarc());
        } else if (proto.hasKeycard()) {
            return new IdentityX509EC(proto.getKeycard());
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
        if (SignerEd25519.class.isInstance(signer)) {
            return new IdentityEd25519(signer);
        } else if (SignerX509EC.class.isInstance(signer)) {
            return new IdentityX509EC(signer);
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
        return new IdentityDarc(darc);
    }
}
