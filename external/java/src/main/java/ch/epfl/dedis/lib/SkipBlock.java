package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import ch.epfl.dedis.skipchain.ForwardLink;
import ch.epfl.dedis.skipchain.SkipchainRPC;
import com.google.protobuf.InvalidProtocolBufferException;

import java.net.URISyntaxException;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

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
     * @throws CothorityException if something goes wrong
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
        MessageDigest digest;
        try {
            digest = MessageDigest.getInstance("SHA-256");
        } catch (NoSuchAlgorithmException e) {
            return null;
        }

        ByteBuffer bb = ByteBuffer.allocate(4);
        bb.order(ByteOrder.LITTLE_ENDIAN);

        bb.putInt(getIndex());
        digest.update(bb.array());
        bb.clear();
        bb.putInt(getHeight());
        digest.update(bb.array());
        bb.clear();
        bb.putInt(getMaximumHeight());
        digest.update(bb.array());
        bb.clear();
        bb.putInt(getBaseHeight());
        digest.update(bb.array());
        bb.clear();

        getBackLinks().forEach(bl -> digest.update(bl.getId()));
        getVerifiers().forEach(digest::update);
        try {
            digest.update(skipBlock.getGenesis().toByteArray());
            digest.update(getData());
            if (getRoster() != null) {
                getRoster().getNodes().forEach(si -> digest.update(si.getPublic().toBytes()));
            }
        } catch (CothorityCryptoException e) {
            return null;
        }

        if (getSignatureScheme() > 0) {
            bb.putInt(getSignatureScheme());
            digest.update(bb.array());
        }

        return digest.digest();
    }

    /**
     * @return the id of the block, which is its hash.
     */
    public SkipblockId getId() {
        return new SkipblockId(this.getHash());
    }

    /**
     * @return the id of the skipchain this block is part of. This is equal to the hash for the
     * genesis block.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public SkipblockId getSkipchainId() throws CothorityCryptoException {
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
        return skipBlock.getIndex();
    }

    /**
     * @return the height of the block.
     */
    public int getHeight() {
        return skipBlock.getHeight();
    }

    /**
     * @return the maximum height of the block.
     */
    public int getMaximumHeight() {
        return skipBlock.getMaxHeight();
    }

    /**
     * @return the base height of the block.
     */
    public int getBaseHeight() {
        return skipBlock.getBaseHeight();
    }

    /**
     * @return the signature scheme index for the block
     */
    public int getSignatureScheme() {
        return skipBlock.getSignatureScheme();
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
     * This function checks whether all signatures in the forward-links
     * are correctly signed by the aggregate public key of the roster
     *
     * @return true if the signature is ok.
     */
    public boolean verifyForwardSignatures() {
        List<Point> publics;
        try {
            publics = new Roster(this.skipBlock.getRoster()).getServicePublics(SkipchainRPC.SERVICE_NANE);
        } catch (URISyntaxException e) {
            return false;
        }

        for (ForwardLink fl : this.getForwardLinks()) {
            if (fl.isEmpty()) {
                // This means it's an empty forward-link to correctly place a higher-order
                // forward-link in place.
                continue;
            }
            if (!fl.verifyWithScheme(publics, getSignatureScheme())) {
                return false;
            }
        }
        return true;
    }

    /**
     * Getter for the list of backlinks in the skipblock.
     */
    public List<SkipblockId> getBackLinks() {
        return skipBlock.getBacklinksList().stream()
                .map(bl -> new SkipblockId(bl.toByteArray()))
                .collect(Collectors.toList());
    }

    /**
     * Getter for the list of verifiers in the skipblock.
     */
    public List<byte[]> getVerifiers() {
        return skipBlock.getVerifiersList().stream()
                .map(v -> v.toByteArray())
                .collect(Collectors.toList());
    }

    /**
     * Gets the roster from the skipblock.
     * @return the roster responsible for that skipblock
     * @throws CothorityCryptoException if the roster cannot be parsed
     */
    public Roster getRoster() throws CothorityCryptoException {
        try {
            return new Roster(skipBlock.getRoster());
        } catch (URISyntaxException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    @Override
    public boolean equals(final java.lang.Object obj) {
        if (obj == this) {
            return true;
        }
        if (!(obj instanceof SkipBlock)) {
            return super.equals(obj);
        }
        SkipBlock other = (SkipBlock) obj;

        return other.getId().equals(this.getId());
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
