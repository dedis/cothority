package ch.epfl.dedis.lib.omniledger.darc;

import ch.epfl.dedis.lib.darc.SignerFactory;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

/**
 * Signature is a darc signature that holds the schnorr signature itself and
 * the identity of the signer.
 */
public class Signature {
    public byte[] signature;
    public Identity signer;

    public Signature(byte[] signature, Identity signer) {
        this.signature = signature;
        this.signer = signer;
    }

    public Signature(DarcProto.Signature sig) throws CothorityCryptoException{
        signature = sig.getSignature().toByteArray();
        signer = IdentityFactory.New(sig.getSigner());
    }

    public DarcProto.Signature toProto() {
        DarcProto.Signature.Builder b = DarcProto.Signature.newBuilder();
        b.setSignature(ByteString.copyFrom(this.signature));
        b.setSigner(this.signer.toProto());
        return b.build();
    }
}
