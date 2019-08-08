package ch.epfl.dedis.byzcoin.transaction;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

public class TxResult {
    private ClientTransaction ct;
    private boolean accepted;

    /**
     * Constructor for TxResult
     * @param proto the input protobuf
     */
    public TxResult(ByzCoinProto.TxResult proto) {
        this(proto, 0);
    }

    /**
     * Create a transaction result from the protobuf-encoded data.
     * @param proto     The data
     * @param version   The protocol version to use
     */
    public TxResult(ByzCoinProto.TxResult proto, int version) {
        ct = new ClientTransaction(proto.getClienttransaction(), version);
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
