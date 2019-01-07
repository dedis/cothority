package ch.epfl.dedis.skipchain;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.List;

/**
 * Implementing an interface to the skipchain service.
 */

public class SkipchainRPC {
    // scID is the id of the skipchain, which is the same as the hash of the genesis block.
    protected SkipblockId scID;

    // the roster that holds the current skipchain
    protected Roster roster;

    private final Logger logger = LoggerFactory.getLogger(SkipchainRPC.class);

    /**
     * If the skipchain is already initialised, this constructor will only
     * initialise the class. Once it is initialized, you can verify it with
     * the verify()-method.
     *
     * @param roster list of all cothority servers with public keys
     * @param scID   the getId of the used skipchain
     * @throws CothorityCommunicationException in case of communication difficulties
     */
    public SkipchainRPC(Roster roster, SkipblockId scID) throws CothorityCommunicationException {
        this.scID = scID;
        this.roster = roster;
    }

    /**
     * Contacts all nodes in the cothority and returns true only if _all_
     * nodes returned OK.
     *
     * @return true only if all nodes are OK, else false.
     */
    public boolean checkStatus() {
        boolean ok = true;
        for (ServerIdentity n : roster.getNodes()) {
            logger.info("Testing node {}", n.getAddress());
            try {
                n.GetStatus();
            } catch (CothorityCommunicationException e) {
                logger.warn("Failing node {}", n.getAddress());
                ok = false;
            }
        }
        return ok;
    }

    /**
     * Returns the skipblock from the skipchain, given its id. Note that the block that is returned is not verified.
     *
     * @param id the id of the skipblock
     * @return the proto-representation of the skipblock.
     * @throws CothorityCommunicationException in case of communication difficulties
     */
    public SkipBlock getSkipblock(SkipblockId id) throws CothorityCommunicationException {
        SkipchainProto.GetSingleBlock request =
                SkipchainProto.GetSingleBlock.newBuilder().setId(ByteString.copyFrom(id.getId())).build();

        ByteString msg = roster.sendMessage("Skipchain/GetSingleBlock",
                request);

        try {
            SkipchainProto.SkipBlock sb = SkipchainProto.SkipBlock.parseFrom(msg);
            SkipBlock ret = new SkipBlock(sb);

            // simple verification (we do not check the links, just the signature)
            if (!ret.verifyForwardSignatures()) {
                throw new CothorityCommunicationException("invalid forward signatures");
            }

            logger.debug("Got the following skipblock: {}", sb);
            logger.info("Successfully read skipblock");

            return ret;
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Returns the latest block and verify that the links are correct.
     *
     * @return the latest skipblock
     * @throws CothorityCommunicationException if something goes wrong with communication
     * @throws CothorityCryptoException if the verification goes wrong
     */
    public SkipBlock getLatestSkipblock() throws CothorityCommunicationException, CothorityCryptoException {
        SkipchainProto.GetUpdateChain request =
                SkipchainProto.GetUpdateChain.newBuilder().setLatestID(ByteString.copyFrom(scID.getId())).build();

        ByteString msg = roster.sendMessage("Skipchain/GetUpdateChain",
                request);

        SkipchainProto.GetUpdateChainReply reply;
        try {
            reply = SkipchainProto.GetUpdateChainReply.parseFrom(msg);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }

        if (reply.getUpdateCount() == 0){
            logger.info("didn't find any updates to {}", scID);
            return null;
        }

        SkipBlock start = new SkipBlock(reply.getUpdateList().get(0));
        if (!scID.equals(start.getId())) {
            throw new CothorityCryptoException("first returned block does not match requested hash");
        }

        /*
        // Step through the returned blocks one at a time, verifying
        // the forward links, and that they link correctly backwards.
        List<SkipBlock> update = new ArrayList<>();
        for (int j = 0; j < reply.getUpdateCount(); j++) {
            SkipBlock b = new SkipBlock(reply.getUpdateList().get(j));
            if (!b.verifyForwardSignatures()) {
                throw new CothorityCryptoException("forward signature verification failed");
            }
            // Cannot check back links until we've confirmed the first one
            if (update.size() > 0) {
                if (b.getBackLinks().size() == 0) {
                    throw new CothorityCryptoException("no backlink");
                }
                SkipBlock prevBlock = update.get(update.size() - 1);
                int link = prevBlock.getHeight();
                if (link > b.getHeight()) {
                    link = b.getHeight();
                }
                // if (b.getBackLinks().)
                // TODO
            }
            update.add(b);
        }
        */

        SkipBlock sb = new SkipBlock(reply.getUpdate(reply.getUpdateCount() - 1));
        logger.info("Got the following latest skipblock: {}", sb);

        return sb;
    }


    public SkipblockId getID() {
        return scID;
    }

    public Roster getRoster() {
        return roster;
    }
}
