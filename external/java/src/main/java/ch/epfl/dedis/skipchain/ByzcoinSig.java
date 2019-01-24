package ch.epfl.dedis.skipchain;

import ch.epfl.dedis.lib.crypto.BlsSig;
import ch.epfl.dedis.lib.crypto.Bn256G2Point;
import ch.epfl.dedis.lib.crypto.Mask;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.bn256.BN;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;

import java.util.Arrays;
import java.util.List;

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
     * @param publics a list of signers
     * @return true if the signature is correct, false otherwise
     */
    public boolean verify(List<Point> publics){
        if (publics == null || publics.size() == 0) {
            // no public keys provided
            return false;
        }
        if (this.getMsg() == null) {
            // no message provided
            return false;
        }
        if (this.getSignature() == null || this.getSignature().length == 0) {
            // no signature provided
            return false;
        }

        int lenCom = BN.G1.MARSHAL_SIZE;
        byte[] signature = Arrays.copyOf(this.getSignature(), lenCom);

        if (lenCom >= this.getSignature().length) {
            // mask is missing
            return false;
        }

        byte[] maskBits = Arrays.copyOfRange(this.getSignature(), lenCom, this.getSignature().length);
        Mask mask;
        try {
            mask = new Mask(publics, maskBits);
        } catch (CothorityCryptoException e) {
            return false;
        }

        BlsSig blsSig = new BlsSig(signature);
        if (!blsSig.verify(this.getMsg(), (Bn256G2Point) mask.getAggregate())) {
            return false;
        }

        // policy default to at >= 3t+1 valid signatures, so make sure we have enough in the mask.
        int n = publics.size();
        int threshold = n - ((n - 1) / 3);
        return mask.countEnabled() >= threshold;
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
