package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.proto.ByzCoinProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.util.ArrayList;
import java.util.List;

public class DataBody {
    public List<TxResult> txResults;

    /**
     * Constructor for DataBody from protobuf.
     * @param proto
     * @throws InvalidProtocolBufferException
     */
    public DataBody(ByzCoinProto.DataBody proto) throws InvalidProtocolBufferException {
        txResults = new ArrayList<TxResult>(proto.getTxresultsCount());
        for (ByzCoinProto.TxResult t : proto.getTxresultsList()) {
            txResults.add(new TxResult(t));
        }
    }
}
