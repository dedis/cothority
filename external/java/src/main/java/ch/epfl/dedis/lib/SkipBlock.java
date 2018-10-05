package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.skipchain.ForwardLink;
import ch.epfl.dedis.lib.proto.SkipchainProto;
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
     * @param skipBlock the protobuf definition of the skipblock.
     */
    public SkipBlock(SkipchainProto.SkipBlock skipBlock) {
        this.skipBlock = skipBlock;
    }

    /**
     * @param sb the binary representation of the protobuf of the skipblock.
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
     * @return the hash of the block, which includes the backward-links and the data.
     */
    public byte[] getHash() {
        return skipBlock.getHash().toByteArray();
    }

    /**
     * @return the id of the block, which is its hash.
     * @throws CothorityCryptoException
     */
    public SkipblockId getId() throws CothorityCryptoException {
        return new SkipblockId(this.getHash());
    }

    /**
     * @return the id of the skipchain this block is part of. This is equal to the hash for the
     * genesis block.
     * @throws CothorityCryptoException
     */
    public SkipblockId getSkipchainId() throws CothorityCryptoException{
        if (skipBlock.getIndex() == 0){
            return getId();
        }
        return new SkipblockId(skipBlock.getGenesis().toByteArray());
    }

    /**
     * @return the data of the block, which is protected by the block hash.
     */
    public byte[] getData(){
        return skipBlock.getData().toByteArray();
    }

    /**
     * @return the payload of the block, which is not directly protected by the block hash.
     */
    public byte[] getPayload() { return skipBlock.getPayload().toByteArray(); }

    /**
     * @return the index of the skipblock - the genesis block has index 0.
     */
    public int getIndex(){
        // Because we're using protobuf's zigzag encoding.
        return skipBlock.getIndex() / 2;
    }

    /**
     * @return the list of all forwardlinks contained in this block. There might be no forward link at all,
     * if this is the tip of the chain.
     */
    public List<ForwardLink>getForwardLinks(){
        List<ForwardLink>ret = new ArrayList<>();
        skipBlock.getForwardList().forEach(fl ->{
            ret.add(new ForwardLink(fl));
        });
        return ret;
    }

    /**
     * Gets the roster from the skipblock.
     * @return the roster responsible for that skipblock
     */
    public Roster getRoster() throws CothorityException {
        try {
            return new Roster(skipBlock.getRoster());
        } catch (URISyntaxException e) {
            throw new CothorityException(e);
        }
    }

    @java.lang.Override
    public boolean equals(final java.lang.Object obj) {
        if (obj == this) {
            return true;
        }
        if (!(obj instanceof SkipBlock)) {
            return super.equals(obj);
        }
        SkipBlock other = (SkipBlock) obj;

        try {
            return other.getId().equals(this.getId());
        } catch (CothorityCryptoException e){
            return false;
        }
    }

    /**
     * @return the serialized skipblock.
     */
    public byte[] toByteArray() {
        return this.skipBlock.toByteArray();
    }

    /**
     * @return the protobuf representation of the block.
     */
    public SkipchainProto.SkipBlock getProto(){
        return skipBlock;
    }
}
