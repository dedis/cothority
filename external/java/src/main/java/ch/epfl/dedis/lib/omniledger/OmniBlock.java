package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.Sha256id;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.OmniLedgerProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

/**
 * OmniBlock represents the data stored in a skipblock that is relevant to OmniLedger. This data is split in
 * two parts:
 * - header, which contains hashes of the current state and which is hashed in the block
 * - body, which contains the actual data (currently ClientTransactions) and which is not directly hashed in the block
 */
public class OmniBlock {
    private SkipBlock skipBlock;
    private OmniLedgerProto.DataHeader dataHeader;
    private OmniLedgerProto.DataBody dataBody;

    /**
     * Instantiates a new OmniBlock given a skipblock.
     *
     * @param sb skipblock holding data for an OmniBLock.
     * @throws CothorityCryptoException
     */
    public OmniBlock(SkipBlock sb) throws CothorityCryptoException {
        try {
            // TODO: check that it is actually an OmniBlock by looking at the verifiers
            dataHeader = OmniLedgerProto.DataHeader.parseFrom(sb.getData());
            dataBody = OmniLedgerProto.DataBody.parseFrom(sb.getPayload());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    /**
     * @return the root hash of the merkle tree of the global state.
     * @throws CothorityCryptoException
     */
    public Sha256id getCollectionRoot() throws CothorityCryptoException {
        return new Sha256id(dataHeader.getCollectionroot());
    }

    /**
     * @return the sha256 of all clientTransactions stored in this block.
     * @throws CothorityCryptoException
     */
    public Sha256id getClientTransactionHash() throws CothorityCryptoException {
        return new Sha256id(dataHeader.getClienttransactionhash());
    }

    /**
     * @return the sha256 hash of all state changes created by the clientTransactions in this block.
     * @throws CothorityCryptoException
     */
    public Sha256id getStateChangesHash() throws CothorityCryptoException {
        return new Sha256id(dataHeader.getStatechangeshash());
    }

    /**
     * @return the unix-timestamp in nanoseconds of the block creation time.
     */
    public long getTimestampNano() {
        return dataHeader.getTimestamp();
    }

    /**
     * @return A java.time.Instant representing the creation of this block.
     */
    public Instant getTimestamp() {
        return Instant.ofEpochMilli(getTimestampNano() / 1000 / 1000);
    }

    /**
     * @return the list of all clientTransactions that went into this block.
     * @throws CothorityCryptoException
     */
    public List<ClientTransaction> getClientTransactions() throws CothorityCryptoException {
        List<ClientTransaction> cts = new ArrayList<>();
        for (OmniLedgerProto.ClientTransaction ct : dataBody.getTransactionsList()) {
            cts.add(new ClientTransaction(ct));
        }
        return cts;
    }
}
