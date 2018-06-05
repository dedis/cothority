package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import ch.epfl.dedis.proto.TransactionProto;
import com.google.protobuf.ByteString;

import java.io.ByteArrayOutputStream;
import java.io.IOException;

/**
 * The object ID of an OmniLedger object. It needs to be unique for every object.
 */
public class ObjectID {
    private DarcId darcId;
    private byte[] instanceId;

    /**
     * The constructor for the ObjectID.
     * @param darcId The darc ID of the object. This darc ID should contain the access rights for the clients that wish
     *               to interact with this object.
     * @param instanceId The instance ID of the object, which must be unique for every object.
     */
    public ObjectID(DarcId darcId, byte[] instanceId) {
        this.darcId = darcId;
        this.instanceId = instanceId;
    }

    /**
     * Getter for the darc ID.
     * @return The darc ID.
     */
    public DarcId getDarcId() {
        return darcId;
    }

    /**
     * Setter for the instance ID.
     * @return The instance ID.
     */
    public byte[] getInstanceId() {
        return instanceId;
    }

    /**
     * Converts this object to a byte array, which is a concatenation of the darc ID and the instance ID.
     * @return The byte array.
     */
    public byte[] toByteArray() {
        try {
            ByteArrayOutputStream os = new ByteArrayOutputStream();
            os.write(this.darcId.getId());
            os.write(this.instanceId);
            return os.toByteArray();
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public TransactionProto.ObjectID toProto() {
        TransactionProto.ObjectID.Builder b = TransactionProto.ObjectID.newBuilder();
        b.setDarcid(ByteString.copyFrom(this.darcId.getId()));
        b.setInstanceid(ByteString.copyFrom(this.instanceId));
        return b.build();
    }
}
