package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.byzcoin.transaction.TxResult;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.util.ArrayList;
import java.util.List;

public class DataBody {
    public List<TxResult> txResults;

    /**
     * Constructor for DataBody from protobuf.
     * @param proto the protobuf form of the DataBody
     * @throws InvalidProtocolBufferException if the DataBody cannot be parsed
     */
    public DataBody(ByzCoinProto.DataBody proto) throws InvalidProtocolBufferException {
        txResults = new ArrayList<TxResult>(proto.getTxresultsCount());
        for (ByzCoinProto.TxResult t : proto.getTxresultsList()) {
            txResults.add(new TxResult(t));
        }
    }
}
