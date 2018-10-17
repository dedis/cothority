package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;

public class TxResult {
    private ClientTransaction ct;
    private boolean accepted;

    /** constructor for TxResult
     *
     * @param proto the input protobuf
     * @throws CothorityCryptoException if there is an issue with the protobuf encoding
     */
    public TxResult(ByzCoinProto.TxResult proto) throws CothorityCryptoException  {
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
