package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.omniledger.darc.Darc;
import ch.epfl.dedis.lib.skipchain.SkipchainRPC;
import ch.epfl.dedis.proto.OmniLedgerProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;

import static java.time.temporal.ChronoUnit.NANOS;

/**
 * Class OmniledgerRPC interacts with the omniledger service of a conode. It can either start a new omniledger service
 * (this needs to be secured somehow) or link to an existing omniledger instance.
 * <p>
 * OmniledgerRPC is the new skipchain service of the cothority that allows batching of transactions and simplified proofs.
 * It is a permissioned blockchain with high throughput (100-1000 transactions) and a byzantine-tolerant consensus
 * algorithm.
 */
public class OmniledgerRPC {
    private Configuration config;
    private Darc genesisDarc;
    private SkipBlock genesis;
    private SkipBlock latest;
    private SkipchainRPC skipchain;
    private static final int currentVersion = 1;

    private final Logger logger = LoggerFactory.getLogger(OmniledgerRPC.class);

    /**
     * This instantiates a new omniledger object by asking the cothority to set up a new omniledger.
     *
     * @param d is the genesis-darc that defines the basic access control to this omniledger
     * @param c is the genesis configuration stored in omniledger
     */
    public OmniledgerRPC(Darc d, Configuration c) throws CothorityException {
        genesisDarc = d;
        config = c;

        OmniLedgerProto.CreateGenesisBlock.Builder request =
                OmniLedgerProto.CreateGenesisBlock.newBuilder();
        request.setVersion(currentVersion);
        request.setRoster(config.getRoster().toProto());
        request.setGenesisdarc(d.toProto());
        request.setBlockinterval(c.getBlockInterval().get(NANOS));

        ByteString msg = config.getRoster().sendMessage("OmniLedger/CreateGenesisBlock",
                request.build());

        try {
            OmniLedgerProto.CreateGenesisBlockResponse reply =
                    OmniLedgerProto.CreateGenesisBlockResponse.parseFrom(msg);
            genesis = new SkipBlock(reply.getSkipblock());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
        latest = genesis;
        logger.info("Created new OmniLedger: {}", genesis.getId().toString());
        skipchain = new SkipchainRPC(config.getRoster(), genesis.getId());
    }

    /**
     * Convenience method for instantiating a new OmniledgerRPC.
     *
     * @param r             is the roster to be used
     * @param d             is the genesis darc
     * @param blockInterval is the block interval between two blocks
     */
    public OmniledgerRPC(Roster r, Darc d, Duration blockInterval) throws CothorityException {
        this(d, new Configuration(r, blockInterval));
    }

    /**
     * Instantiates an omniledger object given the byte representation. The omniledger must already have been
     * initialized on the cothority.
     *
     * @param buf is the representation of the basic omniledger parameters
     */
    public OmniledgerRPC(byte[] buf) {
        throw new RuntimeException("Not implemented yet");
    }

    /**
     * Sends a transaction to omniledger, but doesn't wait for the inclusion of this transaction in a block.
     * Once the transaction has been sent, you need to poll to verify if it has been included or not.
     *
     * @param t is the client transaction holding one or more instructions to be sent to omniledger.
     */
    public void sendTransaction(ClientTransaction t) throws CothorityException {
        OmniLedgerProto.AddTxRequest.Builder request =
                OmniLedgerProto.AddTxRequest.newBuilder();
        request.setVersion(currentVersion);
        request.setSkipchainid(ByteString.copyFrom(skipchain.getID().getId()));
        request.setTransaction(t.toProto());

        ByteString msg = config.getRoster().sendMessage("OmniLedger/AddTxRequest", request.build());
        try{
            OmniLedgerProto.AddTxResponse reply =
                    OmniLedgerProto.AddTxResponse.parseFrom(msg);
            // TODO do something with the reply?
            logger.info("Successfully stored request - waiting for inclusion");
        } catch (InvalidProtocolBufferException e){
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Gets a proof from omniledger to show that a given instance is in the
     * global state.
     *
     * @param id is the id of the instance to be fetched
     * @throws CothorityException
     */
    public Proof getProof(InstanceId id) throws CothorityException {
        OmniLedgerProto.GetProof.Builder request =
                OmniLedgerProto.GetProof.newBuilder();
        request.setVersion(currentVersion);
        request.setId(skipchain.getID().toProto());
        request.setKey(id.toByteString());

        ByteString msg = config.getRoster().sendMessage("OmniLedger/GetProof", request.build());
        try{
            OmniLedgerProto.GetProofResponse reply =
                    OmniLedgerProto.GetProofResponse.parseFrom(msg);
            logger.info("Successfully received proof");
            return new Proof(reply.getProof());
        } catch (InvalidProtocolBufferException e){
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Fetches the latest configuration and genesis darc from omniledger.
     *
     * @throws CothorityException
     */
    public void update() throws CothorityException {
        SkipBlock sb =  skipchain.getLatestSkipblock();
        if (sb != null){
            latest = sb;
        }
    }

    /**
     * Verifies if the nodes representing the cothority are alive and reply to a ping.
     *
     * @return true if all nodes are live, false if one or more are not responding.
     * @throws CothorityException if something failed.
     */
    public boolean checkLiveness() throws CothorityException {
        for (ServerIdentity si : config.getRoster().getNodes()) {
            try {
                logger.info("Checking status of {}", si.getAddress());
                si.GetStatus();
            } catch (CothorityCommunicationException e) {
                logger.warn("Failing node {}: {}", si.getAddress(), e.toString());
                return false;
            }
        }
        return true;
    }

    /**
     * @return a byte representation of this omniledger object.
     */
    public byte[] toBytes() {
        return null;
    }

    /**
     * @return current configuration
     */
    public Configuration getConfig() {
        return config;
    }

    /**
     * @return current genesis darc
     */
    public Darc getGenesisDarc() {
        return genesisDarc;
    }

    /**
     * @return latest skipblock - might be null if omniledger has been instantiated from a buffer and didn't
     * 'update' yet.
     */
    public SkipBlock getLatest() {
        return latest;
    }
}
