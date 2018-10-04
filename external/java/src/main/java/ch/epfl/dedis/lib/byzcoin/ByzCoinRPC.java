package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.byzcoin.contracts.DarcInstance;
import ch.epfl.dedis.lib.byzcoin.darc.Darc;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.skipchain.SkipchainRPC;
import ch.epfl.dedis.proto.ByzCoinProto;
import ch.epfl.dedis.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.ByteBuffer;
import java.time.Duration;

import static java.time.temporal.ChronoUnit.NANOS;

/**
 * Class ByzCoinRPC interacts with the byzcoin service of a conode. It can either start a new byzcoin service
 * (this needs to be secured somehow) or link to an existing byzcoin instance.
 * <p>
 * ByzCoinRPC is the new skipchain service of the cothority that allows batching of transactions and simplified proofs.
 * It is a permissioned blockchain with high throughput (100-1000 transactions) and a byzantine-tolerant consensus
 * algorithm.
 */
public class ByzCoinRPC {
    private Config config;
    private Roster roster;
    private Darc genesisDarc;
    private SkipBlock genesis;
    private SkipBlock latest;
    private SkipchainRPC skipchain;

    private Subscription subscription;
    public static final int currentVersion = 1;

    private final Logger logger = LoggerFactory.getLogger(ByzCoinRPC.class);

    /**
     * This instantiates a new byzcoin object by asking the cothority to set up a new byzcoin.
     *
     * @param r             is the roster to be used
     * @param d             is the genesis darc
     * @param blockInterval is the block interval between two blocks
     */
    public ByzCoinRPC(Roster r, Darc d, Duration blockInterval) throws CothorityException {
        ByzCoinProto.CreateGenesisBlock.Builder request =
                ByzCoinProto.CreateGenesisBlock.newBuilder();
        request.setVersion(currentVersion);
        request.setRoster(r.toProto());
        request.setGenesisdarc(d.toProto());
        request.setBlockinterval(blockInterval.get(NANOS));

        ByteString msg = r.sendMessage("ByzCoin/CreateGenesisBlock",
                request.build());

        try {
            ByzCoinProto.CreateGenesisBlockResponse reply =
                    ByzCoinProto.CreateGenesisBlockResponse.parseFrom(msg);
            genesis = new SkipBlock(reply.getSkipblock());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
        latest = genesis;
        logger.info("Created new ByzCoin ledger with ID: {}", genesis.getId().toString());
        skipchain = new SkipchainRPC(r, genesis.getId());
        config = new Config(blockInterval);
        roster = r;
        genesisDarc = d;
        subscription = new Subscription(this);
    }

    /**
     * Constructs a ByzCoinRPC from a known configuration. The constructor will communicate with the service to
     * populate other fields and perform verification.
     *
     * @param roster      the roster to talk to
     * @param skipchainId the ID of the genesis skipblock, aka skipchain ID
     * @throws CothorityException
     */
    public ByzCoinRPC(Roster roster, SkipblockId skipchainId) throws CothorityException, InvalidProtocolBufferException {
        Proof proof = ByzCoinRPC.getProof(roster, skipchainId, InstanceId.zero());
        if (!proof.isContract("config", skipchainId)) {
            throw new CothorityNotFoundException("couldn't verify proof for genesisConfiguration");
        }
        config = new Config(proof.getValue());

        Proof proof2 = ByzCoinRPC.getProof(roster, skipchainId, new InstanceId(proof.getDarcID().getId()));
        if (!proof2.isContract(DarcInstance.ContractId, skipchainId)) {
            throw new CothorityNotFoundException("couldn't verify proof for genesisConfiguration");
        }
        genesisDarc = new Darc(proof2.getValue());

        // find the skipchain info
        skipchain = new SkipchainRPC(roster, skipchainId);
        this.roster = roster;
        genesis = skipchain.getSkipblock(skipchainId);
        latest = skipchain.getLatestSkipblock();
        subscription = new Subscription(this);
    }

    /**
     * Sends a transaction to byzcoin, but doesn't wait for the inclusion of this transaction in a block.
     * Once the transaction has been sent, you need to poll to verify if it has been included or not.
     *
     * @param t is the client transaction holding one or more instructions to be sent to byzcoin.
     */
    public ClientTransactionId sendTransaction(ClientTransaction t) throws CothorityException {
        return sendTransactionAndWait(t, 0);
    }

    /**
     * Sends a transaction to byzcoin and waits for up to 'wait' blocks for the transaction to be
     * included in the global state. If more than 'wait' blocks are created and the transaction is not
     * included, an exception will be raised.
     *
     * @param t    is the client transaction holding one or more instructions to be sent to byzcoin.
     * @param wait indicates the number of blocks to wait for the transaction to be included.
     * @return ClientTransactionID the transaction ID
     * @throws CothorityException if the transaction has not been included within 'wait' blocks.
     */
    public ClientTransactionId sendTransactionAndWait(ClientTransaction t, int wait) throws CothorityException {
        ByzCoinProto.AddTxRequest.Builder request =
                ByzCoinProto.AddTxRequest.newBuilder();
        request.setVersion(currentVersion);
        request.setSkipchainid(ByteString.copyFrom(skipchain.getID().getId()));
        request.setTransaction(t.toProto());
        request.setInclusionwait(wait);

        ByteString msg = roster.sendMessage("ByzCoin/AddTxRequest", request.build());
        try {
            ByzCoinProto.AddTxResponse reply =
                    ByzCoinProto.AddTxResponse.parseFrom(msg);
            // TODO do something with the reply?
            logger.info("Successfully stored request - waiting for inclusion");
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
        return t.getId();
    }

    /**
     * Gets a proof from byzcoin to show that a given instance is in the
     * global state.
     *
     * @param id is the id of the instance to be fetched
     * @throws CothorityException
     */
    public Proof getProof(InstanceId id) throws CothorityException {
        ByzCoinProto.GetProof.Builder request =
                ByzCoinProto.GetProof.newBuilder();
        request.setVersion(currentVersion);
        request.setId(skipchain.getID().toProto());
        request.setKey(id.toByteString());

        ByteString msg = roster.sendMessage("ByzCoin/GetProof", request.build());
        try {
            ByzCoinProto.GetProofResponse reply =
                    ByzCoinProto.GetProofResponse.parseFrom(msg);
            logger.info("Successfully received proof");
            return new Proof(reply.getProof());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Fetches the latest configuration and genesis darc from byzcoin.
     *
     * @throws CothorityException
     */
    public void update() throws CothorityException {
        SkipBlock sb = skipchain.getLatestSkipblock();
        if (sb != null) {
            latest = sb;
        }
    }

    /**
     * Verifies if the nodes representing the cothority are alive and reply to a ping.
     *
     * @return true if all nodes are live, false if one or more are not responding.
     * @throws CothorityException if something failed.
     */
    public boolean checkLiveness() {
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
     * @return a byte representation of this byzcoin object.
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
     * @return the genesis block of the ledger.
     */
    public SkipBlock getGenesis() {
        return genesis;
    }

    /**
     * @return the roster responsible for the ledger
     */
    public Roster getRoster() {
        return roster;
    }

    /**
     * Fetches a given block from the skipchain and returns the corresponding Block.
     *
     * @param id hash of the skipblock to fetch
     * @return a Block representation of the skipblock
     * @throws CothorityCommunicationException if it couldn't contact the nodes
     * @throws CothorityCryptoException        if the omniblock is invalid
     */
    public Block getBlock(SkipblockId id) throws CothorityCommunicationException, CothorityCryptoException {
        SkipBlock sb = skipchain.getSkipblock(id);
        return new Block(sb);
    }

    /**
     * Fetches the latest block from the Skipchain and returns the corresponding Block.
     *
     * @return a Block representation of the skipblock
     * @throws CothorityCryptoException if the omniblock is invalid
     */
    public Block getLatestBlock() throws CothorityException {
        this.update();
        return new Block(latest);
    }

    /**
     * This should be used with caution. Every time you use this, please open an issue in github and tell us
     * why you think you need this. We'll try to fix it then!
     *
     * @return the underlying skipchain instance.
     */
    public SkipchainRPC getSkipchain() {
        logger.warn("usually you should not need this - please tell us why you do anyway.");
        return skipchain;
    }

    /**
     * Subscribes to all new skipBlocks that might arrive. The subscription is implemented using a polling
     * approach until we have a working streaming solution.
     *
     * @param sbr is a SkipBlockReceiver that will be called with any new block(s) available.
     */
    public void subscribeSkipBlock(Subscription.SkipBlockReceiver sbr) throws CothorityCommunicationException {
        subscription.subscribeSkipBlock(sbr);
    }

    /**
     * Unsubscribes a BlockReceiver.
     *
     * @param sbr the SkipBlockReceiver to unsubscribe.
     */
    public void unsubscribeBlock(Subscription.SkipBlockReceiver sbr) {
        subscription.unsubscribeSkipBlock(sbr);
    }

    /**
     * Static method to request a proof from ByzCoin. This is used in the instantiation method.
     *
     * @param roster      where to contact the cothority
     * @param skipchainId the id of the underlying skipchain
     * @param key         which key we're interested in
     * @return a proof pointing to the instance. The proof can also be a proof that the instance does not exist.
     * @throws CothorityCommunicationException
     */
    private static Proof getProof(Roster roster, SkipblockId skipchainId, InstanceId key) throws CothorityCommunicationException {
        ByzCoinProto.GetProof.Builder configBuilder = ByzCoinProto.GetProof.newBuilder();
        configBuilder.setVersion(currentVersion);
        configBuilder.setId(skipchainId.toProto());
        configBuilder.setKey(key.toByteString());

        ByteString msg = roster.sendMessage("ByzCoin/GetProof", configBuilder.build());

        try {
            ByzCoinProto.GetProofResponse reply = ByzCoinProto.GetProofResponse.parseFrom(msg);
            return new Proof(reply.getProof());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Getter for the subscription object.
     */
    public Subscription getSubscription() {
        return subscription;
    }

    /**
     * Helper function for making the initial connection to the streaming API endpoint.
     *
     * @param receiver contain callbacks that gets called on every response and/or error.
     * @return the streaming connection
     * @throws CothorityCommunicationException
     */
    ServerIdentity.StreamingConn streamTransactions(Subscription.SkipBlockReceiver receiver) throws CothorityCommunicationException {
        ByzCoinProto.StreamingRequest.Builder req = ByzCoinProto.StreamingRequest.newBuilder();
        req.setId(skipchain.getID().toProto());

        ServerIdentity.StreamHandler h = new ServerIdentity.StreamHandler() {
            @Override
            public void receive(ByteBuffer message) {
                try {
                    SkipchainProto.SkipBlock block = ByzCoinProto.StreamingResponse.parseFrom(message).getBlock();
                    receiver.receive(new SkipBlock(block));
                } catch (InvalidProtocolBufferException e) {
                    receiver.error(e.getMessage());
                }
            }

            @Override
            public void error(String s) {
                receiver.error(s);
            }
        };
        return roster.makeStreamingConn("ByzCoin/StreamingRequest", req.build(), h);
    }

}
