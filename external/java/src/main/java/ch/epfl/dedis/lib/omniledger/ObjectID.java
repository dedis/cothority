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

    public ObjectID(DarcId darcId, byte[] instanceId) {
        this.darcId = darcId;
        this.instanceId = instanceId;
    }

    public DarcId getDarcId() {
        return darcId;
    }

    public byte[] getInstanceId() {
        return instanceId;
    }

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

    public TransactionProto.ObjectID toProto() {
        TransactionProto.ObjectID.Builder b = TransactionProto.ObjectID.newBuilder();
        b.setDarcid(ByteString.copyFrom(this.darcId.getId()));
        b.setInstanceid(ByteString.copyFrom(this.instanceId));
        return b.build();
    }
}
