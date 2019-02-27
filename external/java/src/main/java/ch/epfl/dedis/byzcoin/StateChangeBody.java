package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.Sha256id;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;

/**
 * StateChangeBody contains the content of a state change, i.e., everything except the instance ID (the key).
 */
public class StateChangeBody {
    private int stateAction;
    private String contractID;
    private byte[] value;
    private long version;
    private DarcId darcBaseId;

    /**
     * Construct a StateChangeBody object from its protobuf representation.
     */
    public StateChangeBody(ByzCoinProto.StateChangeBody proto) throws CothorityCryptoException {
        stateAction = proto.getStateaction();
        contractID = proto.getContractid();
        value = proto.getValue().toByteArray();
        version = proto.getVersion();
        if (proto.getDarcid().toByteArray().length != Sha256id.length) {
            throw new CothorityCryptoException("darc ID has the wrong length");
        }
        darcBaseId = new DarcId(proto.getDarcid());
    }

    /**
     * Getter for the state action.
     */
    public int getStateAction() {
        return stateAction;
    }

    /**
     * Getter for the contract ID.
     */
    public String getContractID() {
        return contractID;
    }

    /**
     * Getter for the value.
     */
    public byte[] getValue() {
        return value;
    }

    /**
     * Getter for the version
     */
    public long getVersion() {
        return version;
    }

    /**
     * Getter for the Darc base ID.
     */
    public DarcId getDarcBaseId() {
        return darcBaseId;
    }
}
