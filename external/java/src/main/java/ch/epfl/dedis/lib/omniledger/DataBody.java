package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.proto.OmniLedgerProto;
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
    public DataBody(OmniLedgerProto.DataBody proto) throws InvalidProtocolBufferException {
        txResults = new ArrayList<TxResult>(proto.getTxresultsCount());
        for (OmniLedgerProto.TxResult t : proto.getTxresultsList()) {
            txResults.add(new TxResult(t));
        }
    }
}
