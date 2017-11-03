package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;

public class IdentityFactory {
    /**
     * Returns an instantiated identity that is stored in proto.
     */
    public static Identity New(DarcProto.Identity proto) throws Exception{
        if (proto.hasEd25519()){
            return new Ed25519Identity(proto.getEd25519());
        } else if (proto.hasDarc()){
            return new DarcIdentity(proto.getDarc());
        } else {
            throw new Exception("No identity present");
        }
    }

    /**
     * Creates the corresponding identity to a signer.
     *
     * @param signer
     */
    public static Identity New(Signer signer) throws Exception{
        if (Ed25519Signer.class.isInstance(signer)){
            return new Ed25519Identity(signer);
        } else {
            throw new Exception("Cannot make Identity out of " + signer.toString());
        }
    }
}
