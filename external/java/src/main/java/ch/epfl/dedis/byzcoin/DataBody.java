package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.byzcoin.transaction.TxResult;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;

import java.util.ArrayList;
import java.util.List;

public class DataBody {
    private List<TxResult> txResults;

    /**
     * Constructor for DataBody from protobuf.
     * @param proto the protobuf form of the DataBody
     */
    public DataBody(ByzCoinProto.DataBody proto)  {
        this(proto, 0);
    }

    /**
     * Create a body from the protobuf-encoded data.
     * @param proto     The data
     * @param version   The version of the protocol to use
     */
    public DataBody(ByzCoinProto.DataBody proto, int version) {
        txResults = new ArrayList<>(proto.getTxresultsCount());
        for (ByzCoinProto.TxResult t : proto.getTxresultsList()) {
            txResults.add(new TxResult(t, version));
        }
    }

    /**
     * Get the transaction results, which are essentially ClientTransactions with a boolean flag.
     */
    public List<TxResult> getTxResults() {
        return txResults;
    }

}
