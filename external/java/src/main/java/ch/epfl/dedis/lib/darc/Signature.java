package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

public class Signature {
    public byte[] signature;
    public Identity signer;

    public Signature(byte[] signature, Identity signer) {
        this.signature = signature;
        this.signer = signer;
    }

    public DarcProto.Signature toProto() {
        DarcProto.Signature.Builder b = DarcProto.Signature.newBuilder();
        b.setSignature(ByteString.copyFrom(this.signature));
        b.setSigner(this.signer.toProto());
        return b.build();
    }
}
