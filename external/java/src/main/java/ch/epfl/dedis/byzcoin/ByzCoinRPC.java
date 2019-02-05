package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.byzcoin.contracts.ChainConfigData;
import ch.epfl.dedis.byzcoin.contracts.ChainConfigInstance;
import ch.epfl.dedis.byzcoin.contracts.SecureDarcInstance;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.ClientTransactionId;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import ch.epfl.dedis.skipchain.SkipchainRPC;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.ByteBuffer;
import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.Objects;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/**
 * Class ByzCoinRPC interacts with the byzcoin service of a conode. It can either start a new byzcoin service
 * (this needs to be secured somehow) or link to an existing byzcoin service.
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
    final String[] darcContractIDs = new String[]{"secure_darc"};

    private static final Logger logger = LoggerFactory.getLogger(ByzCoinRPC.class);

    /**
     * This instantiates a new byzcoin object by asking the cothority to set up a new byzcoin.
     *
     * @param r             is the roster to be used
     * @param d             is the genesis darc
     * @param blockInterval is the block interval between two blocks
     * @throws CothorityException if something goes wrong
     */
    public ByzCoinRPC(Roster r, Darc d, Duration blockInterval) throws CothorityException {
        if (d.getExpression("invoke:" + ChainConfigInstance.ContractId + ".view_change") == null) {
            throw new CothorityCommunicationException("need a view change rule.");
        }
        ByzCoinProto.CreateGenesisBlock.Builder request =
                ByzCoinProto.CreateGenesisBlock.newBuilder();
        request.setVersion(currentVersion);
        request.setRoster(r.toProto());
        request.setGenesisdarc(d.toProto());
        request.setBlockinterval(blockInterval.toNanos());
        for (int i = 0; i < darcContractIDs.length; i++) {
            request.addDarccontractids(darcContractIDs[i]);
        }

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
     * For use by fromByzcoin
     */
    protected ByzCoinRPC() {
    }

    /**
     * For use by CalypsoRPC
     *
     * @param bc the ByzCoinRPC to copy the config from.
     */
    protected ByzCoinRPC(ByzCoinRPC bc) {
        config = bc.config;
        roster = bc.roster;
        genesisDarc = bc.genesisDarc;
        genesis = bc.genesis;
        latest = bc.latest;
        skipchain = bc.skipchain;
        subscription = bc.subscription;
    }

    /**
     * Sends a transaction to byzcoin, but doesn't wait for the inclusion of this transaction in a block.
     * Once the transaction has been sent, you need to poll to verify if it has been included or not.
     *
     * @param t is the client transaction holding one or more instructions to be sent to byzcoin.
     * @return the client transaction
     * @throws CothorityException if something goes wrong if something goes wrong
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
     * @throws CothorityCommunicationException if the transaction has not been included within 'wait' blocks.
     */
    public ClientTransactionId sendTransactionAndWait(ClientTransaction t, int wait) throws CothorityCommunicationException {
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
     * Gets a proof from byzcoin to show that a given instance is stored in the
     * global state.
     *
     * @param id is the id of the instance to be fetched
     * @return the proof
     * @throws CothorityCommunicationException if something goes wrong
     * @throws CothorityCryptoException if the verification fails
     */
    public Proof getProof(InstanceId id) throws CothorityCommunicationException, CothorityCryptoException {
        ByzCoinProto.GetProof.Builder request =
                ByzCoinProto.GetProof.newBuilder();
        request.setVersion(currentVersion);
        request.setId(skipchain.getID().toProto());
        request.setKey(id.toByteString());

        ByteString msg = roster.sendMessage("ByzCoin/GetProof", request.build());
        try {
            ByzCoinProto.GetProofResponse reply =
                    ByzCoinProto.GetProofResponse.parseFrom(msg);
            Proof p = new Proof(reply.getProof(), skipchain.getID(), id);
            logger.info("Successfully received and created proof");
            return p;
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException("failed to get proof: " + e.getMessage());
        }
    }

    /**
     * Gets the signer counters for the signer IDs. The counters must be set correctly in the instructions for
     * them to be accepted by byzcoin. Every counter maps to a signer, if the most recent instruction is signed by
     * the signer at count n, then the next instruction that the same signer signs must be on counter n+1.
     *
     * @param signerIDs the list of signer IDs
     * @return The corresponding coutners for the given IDs
     * @throws CothorityCommunicationException if something goes wrong
     */
    public SignerCounters getSignerCounters(List<String> signerIDs) throws CothorityCommunicationException {
        ByzCoinProto.GetSignerCounters.Builder b = ByzCoinProto.GetSignerCounters.newBuilder();
        b.addAllSignerids(signerIDs);
        b.setSkipchainid(skipchain.getID().toProto());

        ByteString msg = roster.sendMessage("ByzCoin/GetSignerCounters", b.build());
        try {
            ByzCoinProto.GetSignerCountersResponse reply = ByzCoinProto.GetSignerCountersResponse.parseFrom(msg);
            logger.info("successfully parsed signer counters");
            return new SignerCounters(reply);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Fetches the latest configuration and genesis darc from byzcoin.
     *
     * @throws CothorityException if something goes wrong if something goes wrong
     */
    public void update() throws CothorityException {
        List<SkipBlock> sbs = skipchain.getUpdateChain();
        if (sbs != null && sbs.size() > 0) {
            latest = sbs.get(sbs.size() - 1);
        }
    }

    /**
     * Verifies if the nodes representing the cothority are alive and reply to a ping.
     *
     * @return true if all nodes are live, false if one or more are not responding.
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
     * @return the darc instance of the genesis darc.
     * @throws CothorityException if something goes wrong if something goes wrong
     */
    public SecureDarcInstance getGenesisDarcInstance() throws CothorityException {
        return SecureDarcInstance.fromByzCoin(this, genesisDarc);
    }

    /**
     * @return the genesis block of the ledger.
     */
    public SkipBlock getGenesisBlock() {
        return genesis;
    }

    /**
     * @return the roster responsible for the ledger
     */
    public Roster getRoster() {
        return new Roster(roster.getNodes());
    }

    /**
     * Fetches a given block from the skipchain and returns the corresponding Block.
     *
     * @param id hash of the skipblock to fetch
     * @return a Block representation of the skipblock
     * @throws CothorityCommunicationException if it couldn't contact the nodes
     * @throws CothorityCryptoException        if there's a problem with the cryptography
     */
    public Block getBlock(SkipblockId id) throws CothorityCommunicationException, CothorityCryptoException {
        SkipBlock sb = skipchain.getSkipblock(id);
        return new Block(sb);
    }

    /**
     * Fetches the latest block from the Skipchain and returns the corresponding Block.
     *
     * @return a Block representation of the skipblock
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Block getLatestBlock() throws CothorityException {
        this.update();
        return new Block(latest);
    }

    /**
     * CheckAuthorization asks ByzCoin which of the rules stored in the latest version of the darc given by id
     * can be resolved with a combination of signatures given by identities. Each identity can be of any type. If
     * it is a darc, then any "_sign" rule given by that darc will be accepted.
     *
     * @param id         the base id of the darc to be searched for
     * @param identities a list of identities that might sign
     * @return a list of actions that are allowed by any possible combination of signature from identities
     * @throws CothorityCommunicationException if something goes wrong
     */
    public List<String> checkAuthorization(DarcId id, List<Identity> identities) throws CothorityCommunicationException {
        ByzCoinProto.CheckAuthorization.Builder request =
                ByzCoinProto.CheckAuthorization.newBuilder();
        request.setVersion(currentVersion);
        request.setByzcoinid(ByteString.copyFrom(skipchain.getID().getId()));
        request.setDarcid(ByteString.copyFrom(id.getId()));
        identities.forEach(identity -> request.addIdentities(identity.toProto()));

        ByteString msg = roster.sendMessage("ByzCoin/CheckAuthorization", request.build());
        try {
            ByzCoinProto.CheckAuthorizationResponse reply =
                    ByzCoinProto.CheckAuthorizationResponse.parseFrom(msg);
            logger.info("Got request reply: {}", reply);
            return reply.getActionsList();
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Fetches the state change for the given version and instance
     *
     * @param id      the instance ID
     * @param version the version of the state change
     * @return the state change
     * @throws CothorityCommunicationException if the state change doesn't exist or something went wrong
     */
    public StateChange getInstanceVersion(InstanceId id, long version) throws CothorityCommunicationException {
        ByzCoinProto.GetInstanceVersion.Builder request = ByzCoinProto.GetInstanceVersion.newBuilder();
        request.setInstanceid(id.toByteString());
        request.setSkipchainid(genesis.getId().toProto());
        request.setVersion(version);

        ByteString msg = roster.sendMessage("ByzCoin/GetInstanceVersion", request.build());
        try {
            ByzCoinProto.GetInstanceVersionResponse reply = ByzCoinProto.GetInstanceVersionResponse.parseFrom(msg);

            return new StateChange(reply.getStatechange());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Fetches the most recent state change for the given instance
     *
     * @param id the instance ID
     * @return the state change
     * @throws CothorityCommunicationException if the instance doesn't exist or something went wrong
     */
    public StateChange getLastInstanceVersion(InstanceId id) throws CothorityCommunicationException {
        ByzCoinProto.GetLastInstanceVersion.Builder request = ByzCoinProto.GetLastInstanceVersion.newBuilder();
        request.setInstanceid(id.toByteString());
        request.setSkipchainid(genesis.getId().toProto());

        ByteString msg = roster.sendMessage("ByzCoin/GetLastInstanceVersion", request.build());
        try {
            ByzCoinProto.GetInstanceVersionResponse reply = ByzCoinProto.GetInstanceVersionResponse.parseFrom(msg);

            return new StateChange(reply.getStatechange());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Fetches all the state changes of the given instance
     *
     * @param id the instance ID
     * @return the list of state changes
     * @throws CothorityCommunicationException if the instance doesn't exist or something went wrong
     */
    public List<StateChange> getAllInstanceVersion(InstanceId id) throws CothorityCommunicationException {
        ByzCoinProto.GetAllInstanceVersion.Builder request = ByzCoinProto.GetAllInstanceVersion.newBuilder();
        request.setInstanceid(id.toByteString());
        request.setSkipchainid(genesis.getId().toProto());

        ByteString msg = roster.sendMessage("ByzCoin/GetAllInstanceVersion", request.build());
        try {
            ByzCoinProto.GetAllInstanceVersionResponse reply = ByzCoinProto.GetAllInstanceVersionResponse.parseFrom(msg);

            return reply.getStatechangesList()
                    .stream()
                    .map(sc -> new StateChange(sc.getStatechange()))
                    .collect(Collectors.toList());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Checks if the state change is valid or has been tempered
     *
     * @param sc the state change
     * @return true if the state change is valid
     * @throws CothorityCommunicationException if something went wrong
     */
    public boolean checkStateChangeValidity(StateChange sc) throws CothorityCommunicationException {
        ByzCoinProto.CheckStateChangeValidity.Builder request = ByzCoinProto.CheckStateChangeValidity.newBuilder();
        request.setInstanceid(sc.getInstanceId().toByteString());
        request.setSkipchainid(genesis.getId().toProto());
        request.setVersion(sc.getVersion());

        ByteString msg = roster.sendMessage("ByzCoin/CheckStateChangeValidity", request.build());
        try {
            ByzCoinProto.CheckStateChangeValidityResponse reply =
                    ByzCoinProto.CheckStateChangeValidityResponse.parseFrom(msg);

            StateChanges scs = new StateChanges(reply.getStatechangesList());

            SkipBlock skipblock = skipchain.getSkipblock(new SkipblockId(reply.getBlockid()));
            ByzCoinProto.DataHeader dh = ByzCoinProto.DataHeader.parseFrom(skipblock.getData());

            return scs.getHash().equals(dh.getStatechangeshash());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * This should be used with caution. Every time you use this, please open an issue in github and tell us
     * why you think you need this. We'll try to fix it then!
     *
     * @return the underlying skipchain service.
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
     * @throws CothorityCommunicationException if something goes wrong
     */
    public void subscribeSkipBlock(Subscription.SkipBlockReceiver sbr) throws CothorityCommunicationException {
        subscription.subscribeSkipBlock(sbr);
    }

    /**
     * Subscribe to an infinite stream of future SkipBlocks. Note that you need to give a limit or the
     * thread will hang indefinitely
     * <p>
     * Each subscription request uses its own connection and the stream must be correctly closed to clean
     * the resources
     *
     * @throws CothorityCommunicationException if something goes wrong when establishing the connection
     */
    public Stream<SkipBlock> subscribeSkipBlock() throws CothorityCommunicationException {
        BlockingQueue<SkipBlock> queue = new LinkedBlockingQueue<>();
        ServerIdentity.StreamingConn conn = streamTransactions(queue);

        Stream<SkipBlock> stream = Stream.generate(() -> {
            try {
                return queue.take();
            } catch (InterruptedException e) {
                return null;
            }
        });

        return stream.onClose(conn::close).filter(Objects::nonNull); // we don't want any null in the stream
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
     * Change the current roster of the ByzCoin ledger. You're only allowed to change one node at a time,
     * because the system needs to be able to contact previous nodes. When removing nodes, there is a
     * possibility of future proofs getting bigger, as it will be impossible to create forwardlinks.
     * <p>
     * When adding new nodes, it will take at least two blocks until they are completely active and ready
     * to participate in the consensus. This is due to the time it takes for those blocks to download
     * the global state.
     *
     * @param newRoster a new roster with one addition, one removal or one change
     * @param admins    a list of admins needed to sign off on the change
     * @param adminCtrs a list of monotonically increasing counters for every admin
     * @param wait      how many blocks to wait for the new config to go in
     * @throws CothorityException if something went wrong.
     */
    public void setRoster(Roster newRoster, List<Signer> admins, List<Long> adminCtrs, int wait) throws CothorityException {
        // Verify the new roster is not too different.
        ChainConfigInstance cci = ChainConfigInstance.fromByzcoin(this);
        ChainConfigData ccd = cci.getChainConfig();
        ccd.setRoster(newRoster);
        cci.evolveConfigAndWait(ccd, admins, adminCtrs, wait);
        roster = new Roster(newRoster.getNodes());
    }

    /**
     * Sets the new block interval that ByzCoin uses to create new block. The actual interval between two
     * block in the current implementation is guaranteed to be at least 1 second higher, depending on the
     * network delays and the number of transactions to include.
     * <p>
     * The chosen interval can not be smaller than 5 seconds.
     *
     * @param newInterval how long to wait before starting to assemble a new block
     * @param admins      a list of admins needed to sign off the new configuration
     * @param adminCtrs   a list of monotonically increasing counters for every admin
     * @param wait        how many blocks to wait for the new config to go in
     * @throws CothorityException
     */
    public void setBlockInterval(Duration newInterval, List<Signer> admins, List<Long> adminCtrs, int wait) throws CothorityException {
        ChainConfigInstance cci = ChainConfigInstance.fromByzcoin(this);
        ChainConfigData ccd = cci.getChainConfig();
        ccd.setInterval(newInterval);
        cci.evolveConfigAndWait(ccd, admins, adminCtrs, wait);
    }

    /**
     * Sets the new block interval that ByzCoin uses to create new block. The actual interval between two
     * block in the current implementation is guaranteed to be at least 1 second higher, depending on the
     * network delays and the number of transactions to include.
     * <p>
     * The chosen interval can not be smaller than 5 seconds.
     *
     * @param newMaxSize new maximum size of the assembled blocks.
     * @param admins     a list of admins needed to sign off the new configuration
     * @param adminCtrs  a list of monotonically increasing counters for every admin
     * @param wait       how many blocks to wait for the new config to go in
     * @throws CothorityException
     */
    public void setMaxBlockSize(int newMaxSize, List<Signer> admins, List<Long> adminCtrs, int wait) throws CothorityException {
        ChainConfigInstance cci = ChainConfigInstance.fromByzcoin(this);
        ChainConfigData ccd = cci.getChainConfig();
        ccd.setMaxBlockSize(newMaxSize);
        cci.evolveConfigAndWait(ccd, admins, adminCtrs, wait);
    }

    /**
     * Constructs a ByzCoinRPC from a known configuration. The constructor will communicate with the service to
     * populate other fields and perform verification.
     *
     * @param roster      the roster to talk to
     * @param skipchainId the ID of the genesis skipblock, aka skipchain ID
     * @return a new ByzCoinRPC object, connected to the requested roster and chain.
     * @throws CothorityException if something goes wrong
     */
    public static ByzCoinRPC fromByzCoin(Roster roster, SkipblockId skipchainId) throws CothorityException {
        Proof proof = ByzCoinRPC.getProof(roster, skipchainId, InstanceId.zero());
        if (!proof.contractIsType("config")) {
            throw new CothorityNotFoundException("couldn't verify proof for genesisConfiguration");
        }
        if (!proof.exists(InstanceId.zero().getId())) {
            throw new CothorityNotFoundException("config instance does not exist");
        }
        ByzCoinRPC bc = new ByzCoinRPC();
        bc.config = new Config(proof.getValue());

        Proof proof2 = ByzCoinRPC.getProof(roster, skipchainId, new InstanceId(proof.getDarcBaseID().getId()));
        if (!proof2.contractIsType(SecureDarcInstance.ContractId)) {
            throw new CothorityNotFoundException("couldn't verify proof for genesisConfiguration");
        }
        if (!proof2.exists(proof.getDarcBaseID().getId())) {
            throw new CothorityNotFoundException("darc instance does not exist");
        }
        try {
            bc.genesisDarc = new Darc(proof2.getValue());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException("couldn't get genesis darc: " + e.getMessage());
        }

        // find the skipchain info
        bc.skipchain = new SkipchainRPC(roster, skipchainId);
        bc.roster = roster;
        bc.genesis = bc.skipchain.getSkipblock(skipchainId);
        bc.subscription = new Subscription(bc);
        List<SkipBlock> sbs = bc.skipchain.getUpdateChain();
        bc.latest = sbs.get(sbs.size() - 1);
        return bc;
    }

    /**
     * Creates a genesis darc to use for the initialisation of Byzcoin.
     *
     * @param admin  the admin of Byzcoin
     * @param roster the nodes of Byzcoin
     * @return a Darc with the correct rights, also for the view_change.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public static Darc makeGenesisDarc(Signer admin, Roster roster) throws CothorityCryptoException {
        Darc d = new Darc(Arrays.asList(admin.getIdentity()), Arrays.asList(admin.getIdentity()), "Genesis darc".getBytes());
        roster.getNodes().forEach(node -> {
            try {
                d.addIdentity("invoke:"  + ChainConfigInstance.ContractId + ".view_change", new IdentityEd25519((Ed25519Point) node.getPublic()), Rules.OR);
            } catch (CothorityCryptoException e) {
                logger.warn("didn't find Ed25519 point");
            }
        });
        d.addIdentity("spawn:" + SecureDarcInstance.ContractId, admin.getIdentity(), Rules.OR);
        d.addIdentity("invoke:" + ChainConfigInstance.ContractId + ".update_config", admin.getIdentity(), Rules.OR);
        d.addIdentity("invoke:" + SecureDarcInstance.ContractId + ".evolve_unrestricted", admin.getIdentity(), Rules.OR);
        return d;
    }

    /**
     * Static method to request a proof from ByzCoin. This is used in the instantiation method.
     * The returned proof is not verified. Please call Proof.verify.
     *
     * @param roster      where to contact the cothority
     * @param skipchainId the id of the underlying skipchain
     * @param key         which key we're interested in
     * @return a proof pointing to the instance. The proof can also be a proof that the instance does not exist.
     * @throws CothorityCommunicationException if there is an error in getting the proof
     * @throws CothorityCryptoException if there is an issue with the validity of the proof
     */
    private static Proof getProof(Roster roster, SkipblockId skipchainId, InstanceId key) throws CothorityCommunicationException, CothorityCryptoException {
        ByzCoinProto.GetProof.Builder configBuilder = ByzCoinProto.GetProof.newBuilder();
        configBuilder.setVersion(currentVersion);
        configBuilder.setId(skipchainId.toProto());
        configBuilder.setKey(key.toByteString());

        ByteString msg = roster.sendMessage("ByzCoin/GetProof", configBuilder.build());

        try {
            ByzCoinProto.GetProofResponse reply = ByzCoinProto.GetProofResponse.parseFrom(msg);
            return new Proof(reply.getProof(), skipchainId, key);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Getter for the subscription object.
     *
     * @return the Subscription
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

    /**
     * Helper function to make a connection to the streaming API endpoint and populate a blocking queue with the
     * new blocks. The queue will be used during a stream generation
     * <p>
     * As a stream doesn't handle errors, they are ignored and written in the logs.
     *
     * @param queue The blocking queue used by the stream
     * @throws CothorityCommunicationException if something goes wrong when establishing the connection
     */
    private ServerIdentity.StreamingConn streamTransactions(BlockingQueue<SkipBlock> queue) throws CothorityCommunicationException {
        ByzCoinProto.StreamingRequest.Builder req = ByzCoinProto.StreamingRequest.newBuilder();
        req.setId(skipchain.getID().toProto());

        ServerIdentity.StreamHandler h = new ServerIdentity.StreamHandler() {
            @Override
            public void receive(ByteBuffer message) {
                try {
                    SkipchainProto.SkipBlock block = ByzCoinProto.StreamingResponse.parseFrom(message).getBlock();
                    queue.add(new SkipBlock(block));
                } catch (InvalidProtocolBufferException e) {
                    // ignore invalid block but keep a log of the event
                    logger.error(e.getMessage());
                }
            }

            @Override
            public void error(String s) {
                logger.error(s);
            }
        };

        return roster.makeStreamingConn("ByzCoin/StreamingRequest", req.build(), h);
    }

}
