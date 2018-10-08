package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.InvalidProtocolBufferException;

public class TxResult {
    private ClientTransaction ct;
    private boolean accepted;

    /** constructor for TxResult
     *
     * @param proto the input protobuf
     * @throws InvalidProtocolBufferException if the input cannot be parsed
     */
    public TxResult(ByzCoinProto.TxResult proto) throws InvalidProtocolBufferException {
        ct = new ClientTransaction(proto.getClienttransaction());
        accepted = proto.getAccepted();
    }

    /**
     * Getter for the client transaction.
     * @return a client transaction
     */
    public ClientTransaction getClientTransaction() {
        return ct;
    }

    /**
     * isAccepted shows whether this transaction was accepted or rejected in this block.
     * @return true if accepted
     */
    public boolean isAccepted() {
        return accepted;
    }

}
