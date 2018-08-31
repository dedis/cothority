package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.OmniLedgerProto;
import com.google.protobuf.InvalidProtocolBufferException;

public class TxResult {
    private ClientTransaction ct;
    private boolean accepted;

    /** constructor for TxResult
     *
     * @param proto
     * @throws InvalidProtocolBufferException
     */
    public TxResult(OmniLedgerProto.TxResult proto) throws InvalidProtocolBufferException {
        ct = new ClientTransaction(proto.getClienttransaction());
        accepted = proto.getAccepted();
    }

    /**
     * Getter for the client transaction.
     * @return
     */
    public ClientTransaction getClientTransaction() {
        return ct;
    }

    /**
     * isAccepted shows whether this transaction was accepted or rejected in this block.
     * @return
     */
    public boolean isAccepted() {
        return accepted;
    }

}
