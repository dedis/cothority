package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.DarcProto;

public class IdentityFactory {
    /**
     * Returns an instantiated identity that is stored in proto.
     * @param proto the protobuf form of the Identity
     * @return the new Identity
     */
    public static Identity New(DarcProto.Identity proto) {
        if (proto.hasEd25519()) {
            return new IdentityEd25519(proto.getEd25519());
        } else if (proto.hasDarc()) {
            return new IdentityDarc(proto.getDarc());
        } else if (proto.hasX509Ec()) {
            return new IdentityX509EC(proto.getX509Ec());
        } else {
            throw new RuntimeException("No identity present");
        }
    }

    /**
     * Creates the corresponding identity to a signer.
     *
     * @param signer the input signer
     * @return the new Identity
     */
    public static Identity New(Signer signer) {
        if (SignerEd25519.class.isInstance(signer)) {
            return new IdentityEd25519(signer);
        } else if (SignerX509EC.class.isInstance(signer)) {
            return new IdentityX509EC(signer);
        } else {
            throw new RuntimeException("Cannot make Identity out of " + signer.toString());
        }
    }

    /**
     * Creates the corresponding identity to a darc.
     *
     * @param darc the input Darc
     * @return the new Identity
     */
    public static Identity New(Darc darc) {
        return new IdentityDarc(darc);
    }
}
