package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.darc.Darc;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.proto.OmniLedgerProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;

/**
 * The OmniLedger class offers the ability to create a new skipchain, and perform primitive operations on it such as
 * adding a simple key/value pair and retrieving it. For application specific use-cases, please see the classes that
 * inherits OmniLedger, such as EventLog.
 */
public class OmniLedger {
    private static final int currentVersion = 1;
    private Roster roster;
    private Darc genesisDarc;
    private long blockInterval;
    private byte[] skipchainID;

    /**
     *
     * @param roster
     * @param genesisDarc
     * @param blockInterval
     * @throws CothorityException
     */
    public OmniLedger(Roster roster, Darc genesisDarc, long blockInterval) throws CothorityException {
        byte[] sid = create(roster, genesisDarc, blockInterval);
        this.roster = roster;
        this.genesisDarc = genesisDarc;
        this.blockInterval = blockInterval;
        this.skipchainID = sid;
    }

    public OmniLedger(Roster roseter, Darc genesisDarc, long blockInterval, byte[] sid) throws CothorityException {
        check(roster, genesisDarc, blockInterval, skipchainID);
        this.roster = roster;
        this.genesisDarc = genesisDarc;
        this.blockInterval = blockInterval;
        this.skipchainID = sid;
    }

    /**
     * Loads the OmniLedger configuration from a file.
     * @param filepath
     * @throws CothorityException
     */
    public OmniLedger(String filepath) throws CothorityException {
        // parse the config, if genesis block exists, call the check method, otherwise call the create method
        throw new CothorityException("not implemented");
    }

    public long addTransaction(ClientTransaction tx) throws CothorityCommunicationException {
        OmniLedgerProto.AddTxRequest.Builder b = OmniLedgerProto.AddTxRequest.newBuilder();
        b.setVersion(OmniLedger.currentVersion);
        b.setSkipchainid(ByteString.copyFrom(this.skipchainID));
        b.setTransaction(tx.toProto());

        ByteString msg = roster.sendMessage("OmniLedger/AddTxRequest", b.build());

        try {
            OmniLedgerProto.AddTxResponse resp = OmniLedgerProto.AddTxResponse.parseFrom(msg);
            return resp.getVersion();
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    public void getProof(byte[] key) throws CothorityCommunicationException {
        throw new CothorityCommunicationException("not implemented");
    }

    public void saveConfig(String fileName) {
        // TODO suppose we have a OmniLedgerConfig protobuf struct, populate all the fields, serialise the struct, and
        // store it on the file.
    }

    private static byte[] create(Roster roster, Darc genesisDarc, long blockInterval) throws CothorityCommunicationException  {
        OmniLedgerProto.CreateGenesisBlock.Builder b = OmniLedgerProto.CreateGenesisBlock.newBuilder();
        b.setVersion(OmniLedger.currentVersion);
        b.setRoster(roster.toProto());
        b.setGenesisdarc(genesisDarc.toProto());
        b.setBlockinterval(blockInterval);

        ByteString msg = roster.sendMessage("OmniLedger/CreateGenesisBlock", b.build());

        try {
            OmniLedgerProto.CreateGenesisBlockResponse reply = OmniLedgerProto.CreateGenesisBlockResponse.parseFrom(msg);
            return reply.getSkipblock().getHash().toByteArray();
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    private static void check(Roster roster, Darc genesisDarc, long blockInterval, byte[] skipchainID) throws CothorityCommunicationException {
        // try to read the genesis darc and check whether the information inside matches with the input arguments
        throw new CothorityCommunicationException("not implemented");
    }
}
