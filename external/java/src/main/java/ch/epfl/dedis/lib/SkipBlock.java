package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.proto.SkipchainProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.net.URISyntaxException;

/**
 * SkipBlock is a wrapper around the protobuf SkipBlock class. It is mainly used to serialize the genesis block for
 * storage.
 */
public class SkipBlock {
    private SkipchainProto.SkipBlock skipBlock;

    public SkipBlock(SkipchainProto.SkipBlock skipBlock) {
        this.skipBlock = skipBlock;
    }

    public SkipBlock(byte[] sb) throws CothorityException {
        try {
            this.skipBlock = SkipchainProto.SkipBlock.parseFrom(sb);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityException(e);
        }
    }

    public SkipchainProto.SkipBlock getProto(){
        return skipBlock;
    }

    /**
     * Returns the serialized skipblock.
     */
    public byte[] toByteArray() {
        return this.skipBlock.toByteArray();
    }

    public byte[] getHash() {
        return skipBlock.getHash().toByteArray();
    }

    public SkipblockId getId() throws CothorityCryptoException {
        return new SkipblockId(this.getHash());
    }

    public SkipblockId getSkipchainId() throws CothorityCryptoException{
        if (skipBlock.getIndex() == 0){
            return getId();
        }
        return new SkipblockId(skipBlock.getGenesis().toByteArray());
    }

    public byte[] getData(){
        return skipBlock.getData().toByteArray();
    }

    public int getIndex(){
        // Because we're using protobuf's zigzag encoding.
        return skipBlock.getIndex() / 2;
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

