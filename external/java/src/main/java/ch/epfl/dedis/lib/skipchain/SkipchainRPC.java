package ch.epfl.dedis.lib.skipchain;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.proto.SkipBlockProto;
import ch.epfl.dedis.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

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
    public boolean verify() {
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
     * Returns the skipblock from the skipchain, given its id.
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
            SkipBlockProto.SkipBlock sb = SkipBlockProto.SkipBlock.parseFrom(msg);
            //TODO: add verification that the skipblock is valid by hashing and comparing to the id

            logger.debug("Got the following skipblock: {}", sb);
            logger.info("Successfully read skipblock");

            return new SkipBlock(sb);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    public SkipBlock getLatestSkipblock() throws CothorityCommunicationException {
        SkipchainProto.GetUpdateChain request =
                SkipchainProto.GetUpdateChain.newBuilder().setLatestID(ByteString.copyFrom(scID.getId())).build();

        ByteString msg = roster.sendMessage("Skipchain/GetUpdateChain",
                request);

        try {
            SkipchainProto.GetUpdateChainReply reply =
                    SkipchainProto.GetUpdateChainReply.parseFrom(msg);
            //TODO: add verification that the skipblock is valid by hashing and comparing to the id

            if (reply.getUpdateCount() == 0){
                logger.info("didn't find any updates to {}", scID);
                return null;
            }
            SkipBlock sb = new SkipBlock(reply.getUpdate(reply.getUpdateCount() - 1));
            logger.info("Got the following latest skipblock: {}", sb);

            return sb;
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }


    public SkipblockId getID() {
        return scID;
    }

    public Roster getRoster() {
        return roster;
    }
}
