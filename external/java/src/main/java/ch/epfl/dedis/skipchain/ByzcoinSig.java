package ch.epfl.dedis.skipchain;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;

/**
 * ByzcoinSig represents a signature from the byzcoin-protocol. It holds both the message and the signature.
 */
public class ByzcoinSig {
    private SkipchainProto.ByzcoinSig byzcoinSig;

    public ByzcoinSig(SkipchainProto.ByzcoinSig bs){
        byzcoinSig = bs;
    }

    public ByzcoinSig(byte[] buf) throws InvalidProtocolBufferException {
        byzcoinSig = SkipchainProto.ByzcoinSig.parseFrom(buf);
    }

    public ByzcoinSig(ByteString bs) throws InvalidProtocolBufferException{
        byzcoinSig = SkipchainProto.ByzcoinSig.parseFrom(bs);
    }

    /**
     * Verifies the signature given a roster of potential signers.
     * @param roster a list of signers
     * @return true if the signature is correct, false otherwise
     */
    public boolean verify(Roster roster){
        throw new RuntimeException("Not implemented yet");
    }

    /**
     * @return the signed message, it's a sha256 hash of the actual message.
     */
    public byte[] getMsg(){
        return byzcoinSig.getMsg().toByteArray();
    }

    /**
     * @return the aggregated schnorr signature on the message, plus a filter to indicate which nodes signed.
     */
    public byte[] getSignature(){
        return byzcoinSig.getSig().toByteArray();
    }
}
