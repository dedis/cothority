package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.omniledger.darc.Darc;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
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
    private Config config;
    private Roster roster;
    private Darc genesisDarc;
    private SkipBlock genesis;
    private SkipBlock latest;
    private SkipchainRPC skipchain;
    public static final int currentVersion = 1;

    private final Logger logger = LoggerFactory.getLogger(OmniledgerRPC.class);

    /**
     * This instantiates a new omniledger object by asking the cothority to set up a new omniledger.
     *
     * @param r             is the roster to be used
     * @param d             is the genesis darc
     * @param blockInterval is the block interval between two blocks
     */
    public OmniledgerRPC(Roster r, Darc d, Duration blockInterval) throws CothorityException {
        OmniLedgerProto.CreateGenesisBlock.Builder request =
                OmniLedgerProto.CreateGenesisBlock.newBuilder();
        request.setVersion(currentVersion);
        request.setRoster(r.toProto());
        request.setGenesisdarc(d.toProto());
        request.setBlockinterval(blockInterval.get(NANOS));

        ByteString msg = r.sendMessage("OmniLedger/CreateGenesisBlock",
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
        skipchain = new SkipchainRPC(r, genesis.getId());
        config = new Config(blockInterval);
        roster = r;
        genesisDarc = d;
    }

    /**
     * Constructs an OmniLedgerRPC from known configuration. The constructor will communicate with the service to
     * populate other fields and perform verification.
     *
     * @param roster - the roster to talk to
     * @param skipchainId - the ID of the genesis skipblock, aka skipchain ID
     * @throws CothorityException
     */
    public OmniledgerRPC(Roster roster, SkipblockId skipchainId) throws CothorityException {
        // find the darc ID
        Proof proof = OmniledgerRPC.getProof(roster, skipchainId, InstanceId.zero());
        OmniledgerRPC.checkProof(proof, "config");
        DarcId darcId = new DarcId(proof.getValues().get(0));

        // find the actual darc
        InstanceId darcInstanceId = new InstanceId(darcId, SubId.zero());
        proof = OmniledgerRPC.getProof(roster, skipchainId, darcInstanceId);
        OmniledgerRPC.checkProof(proof, "darc");
        try {
            genesisDarc = new Darc(proof.getValues().get(0));
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }

        // find the config info
        InstanceId configInstanceId = new InstanceId(darcId, SubId.one());
        proof = OmniledgerRPC.getProof(roster, skipchainId, configInstanceId);
        OmniledgerRPC.checkProof(proof, "config");
        config = new Config(proof.getValues().get(0));

        // find the skipchain info
        skipchain = new SkipchainRPC(roster, skipchainId);
        this.roster = roster;
        genesis = skipchain.getSkipblock(skipchainId);
        latest = skipchain.getLatestSkipblock();
    }

    /**
     * Instantiates an omniledger object given the byte representation. The omniledger must already have been
     * initialized on the cothority.
     *
     * @param buf is the representation of the basic omniledger parameters, it should have a Roster and a skipchain ID.
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

        ByteString msg = roster.sendMessage("OmniLedger/AddTxRequest", request.build());
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

        ByteString msg = roster.sendMessage("OmniLedger/GetProof", request.build());
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
        for (ServerIdentity si : roster.getNodes()) {
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
    public Config getConfig() {
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

    public SkipBlock getGenesis() {
        return genesis;
    }

    public Roster getRoster() {
        return roster;
    }

    private static void checkProof(Proof proof, String expectedContract) throws CothorityNotFoundException {
        if (!proof.matches()) {
            throw new CothorityNotFoundException("couldn't find darc");
        }
        if (proof.getValues().size() != 2) {
            throw new CothorityNotFoundException("incorrect number of values in proof");
        }
        String contract = new String(proof.getValues().get(1));
        if (!contract.equals(expectedContract)) {
            throw new CothorityNotFoundException("contract name is not " + expectedContract + ", got " + contract);
        }
    }

    private static Proof getProof(Roster roster, SkipblockId skipchainId, InstanceId key) throws CothorityCommunicationException {
        OmniLedgerProto.GetProof.Builder configBuilder = OmniLedgerProto.GetProof.newBuilder();
        configBuilder.setVersion(currentVersion);
        configBuilder.setId(skipchainId.toProto());
        configBuilder.setKey(key.toByteString());

        ByteString msg = roster.sendMessage("OmniLedger/GetProof", configBuilder.build());

        try {
            OmniLedgerProto.GetProofResponse reply = OmniLedgerProto.GetProofResponse.parseFrom(msg);
            return new Proof(reply.getProof());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }
}
