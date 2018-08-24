package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.skipchain.ForwardLink;
import ch.epfl.dedis.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;

import java.net.URISyntaxException;
import java.util.ArrayList;
import java.util.List;

/**
 * SkipBlock is a wrapper around the protobuf SkipBlock class. It is mainly used to serialize the genesis block for
 * storage.
 */
public class SkipBlock {
    private SkipchainProto.SkipBlock skipBlock;

    /**
     * Instantiates a new skipblock given its protobuf representation.
     *
     * @param skipBlock the protobuf representation of the skipblock.
     */
    public SkipBlock(SkipchainProto.SkipBlock skipBlock) {
        this.skipBlock = skipBlock;
    }

    /**
     * Instantiates a new skipblock given its binary representation.
     *
     * @param sb the binary representation of the skipblock.
     * @throws CothorityException
     */
    public SkipBlock(byte[] sb) throws CothorityException {
        try {
            this.skipBlock = SkipchainProto.SkipBlock.parseFrom(sb);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityException(e);
        }
    }

    /**
     * @return the protobuf representation of the skipblock.
     */
    public SkipchainProto.SkipBlock getProto() {
        return skipBlock;
    }

    /**
     * @return the serialized skipblock.
     */
    public byte[] toByteArray() {
        return this.skipBlock.toByteArray();
    }

    /**
     * @return the hash of the skipblock - this only includes the header, not an eventual payload.
     */
    public byte[] getHash() {
        return skipBlock.getHash().toByteArray();
    }

    /**
     * @return the id of the skipblock, which is its hash.
     * @throws CothorityCryptoException
     */
    public SkipblockId getId() throws CothorityCryptoException {
        return new SkipblockId(this.getHash());
    }

    /**
     * @return the id of the skipchain this block belongs to. This is the hash of the genesis block.
     * @throws CothorityCryptoException
     */
    public SkipblockId getSkipchainId() throws CothorityCryptoException {
        if (skipBlock.getIndex() == 0) {
            return getId();
        }
        return new SkipblockId(skipBlock.getGenesis().toByteArray());
    }

    /**
     * @return the data of the skipblock. This is included in the hash.
     */
    public byte[] getData() {
        return skipBlock.getData().toByteArray();
    }

    /**
     * @return the payload of the skipblock. This is not included in the hash.
     */
    public byte[] getPayload() {
        return skipBlock.getPayload().toByteArray();
    }

    /**
     * @return the index of the skipblock. Index == 0 is the genesis block. All other blocks have monotonically increasing
     * indexes.
     */
    public int getIndex() {
        // Because we're using protobuf's zigzag encoding.
        return skipBlock.getIndex() / 2;
    }

    /**
     * @return a list of backlinkIDs that point to previous blocks.
     * @throws CothorityCryptoException
     */
    public List<SkipblockId> getBacklinks() throws CothorityCryptoException{
        List<SkipblockId> sbids = new ArrayList<>();
        for (ByteString sbid : skipBlock.getBacklinksList()) {
            sbids.add(new SkipblockId(sbid));
        }
        return sbids;
    }

    /**
     * @return a list of forwardlinks that point to future blocks - might be empty in case it's the last block.
     * @throws CothorityCryptoException
     */
    public List<ForwardLink> getForwardLinks() throws CothorityCryptoException{
        List<ForwardLink> fls = new ArrayList<>();
        for (SkipchainProto.ForwardLink fl: skipBlock.getForwardList()){
            fls.add(new ForwardLink(fl));
        }
        return fls;
    }

    /**
     * Gets the roster from the skipblock.
     */
    public Roster getRoster() throws CothorityException {
        try {
            return new Roster(skipBlock.getRoster());
        } catch (URISyntaxException e) {
            throw new CothorityException(e);
        }
    }
}

