package ch.epfl.dedis.skipchain;

import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;

import java.net.URISyntaxException;

/**
 * A forwardlink represents a signed proof that a future block has been accepted by the set of nodes of
 * the current block.
 */
public class ForwardLink {
    private SkipchainProto.ForwardLink forwardLink;

    public ForwardLink(SkipchainProto.ForwardLink fl){
        forwardLink = fl;
    }

    public ForwardLink(byte[] buf) throws InvalidProtocolBufferException{
        forwardLink = SkipchainProto.ForwardLink.parseFrom(buf);
    }

    public ForwardLink(ByteString bs) throws InvalidProtocolBufferException{
        forwardLink = SkipchainProto.ForwardLink.parseFrom(bs);
    }

    /**
     * @return the block where this link originates.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public SkipblockId getFrom() throws CothorityCryptoException{
        return new SkipblockId(forwardLink.getFrom());
    }

    /**
     * @return the block where this link points to.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public SkipblockId getTo() throws CothorityCryptoException{
        return new SkipblockId(forwardLink.getTo());
    }

    /**
     * @return the new roster of the 'to' block. If the roster of 'to' and 'from' are the same, this returns null.
     * @throws URISyntaxException if the roster in the forward link has a problem
     */
    public Roster getNewRoster() throws URISyntaxException{
        if (forwardLink.hasNewRoster()) {
            return new Roster(forwardLink.getNewRoster());
        }
        return null;
    }

    /**
     * @return the byzcoin signature on the concatenated 'from', 'to', and 'newRoster', if present.
     */
    public ByzcoinSig getByzcoinSig(){
        return new ByzcoinSig(forwardLink.getSignature());
    }
}
