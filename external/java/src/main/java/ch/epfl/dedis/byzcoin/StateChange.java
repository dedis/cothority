package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.ByteString;

/**
 * Represents the state change of an instance and thus contains all the
 * information related.
 */
public class StateChange {
    private StateAction stateAction;
    private InstanceId instanceId;
    private String contractId;
    private DarcId darcBaseID;
    private ByteString value;
    private long version;

    /**
     * Instantiates using a state change coming from a protobuf message
     * @param sc the state change
     */
    public StateChange(ByzCoinProto.StateChange sc) {
        instanceId = new InstanceId(sc.getInstanceid());
        value = sc.getValue();
        version = sc.getVersion();
        contractId = sc.getContractid();

        if (!sc.getDarcid().isEmpty()) {
            darcBaseID = new DarcId(sc.getDarcid());
        }

        stateAction = StateAction.fromInteger(sc.getStateaction());
    }

    /**
     * Getter for the state action
     * @return the state action
     */
    public StateAction getStateAction() {
        return stateAction;
    }

    /**
     * Getter for the instance ID
     * @return the instance ID
     */
    public InstanceId getInstanceId() {
        return instanceId;
    }

    /**
     * Getter for the contract ID
     * @return the contract ID
     */
    public String getContractId() {
        return contractId;
    }

    /**
     * Getter for the darc ID
     * @return the darc ID
     */
    public DarcId getDarcBaseID() {
        return darcBaseID;
    }

    /**
     * Getter for the value
     * @return the value
     */
    public ByteString getValue() {
        return value;
    }

    /**
     * Getter for the version
     * @return the version
     */
    public long getVersion() {
        return version;
    }

    /**
     * Enumeration to represents the state action
     */
    public enum StateAction {
        Create,
        Update,
        Remove,
        Unknown;

        public static StateAction fromInteger(int i) {
            switch(i) {
                case 1:
                    return Create;
                case 2:
                    return Update;
                case 3:
                    return Remove;
                default:
                    return Unknown;
            }
        }
    }
}
