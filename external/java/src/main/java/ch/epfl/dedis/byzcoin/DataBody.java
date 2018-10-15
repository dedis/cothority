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
     * @throws CothorityCryptoException if there is a problem with the encoding
     */
    public DataBody(ByzCoinProto.DataBody proto) throws CothorityCryptoException  {
        txResults = new ArrayList<>(proto.getTxresultsCount());
        for (ByzCoinProto.TxResult t : proto.getTxresultsList()) {
            txResults.add(new TxResult(t));
        }
    }

    /**
     * Get the transaction results, which are essentially ClientTransactions with a boolean flag.
     */
    public List<TxResult> getTxResults() {
        return txResults;
    }

}
