package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.proto.Calypso;

/**
 * Reply of the CreateLTS RPC call.
 */
public class CreateLTSReply {
    private Point X;
    private byte[] ltsID;

    /**
     * The copy constructor.
     */
    public CreateLTSReply(CreateLTSReply other) {
        this.X = other.getX();
        this.ltsID = other.getLtsID();
    }

    /**
     * Construct from the protobuf object.
     */
    public CreateLTSReply(Calypso.CreateLTSReply proto) {
        this.X = new Ed25519Point(proto.getX().toByteArray());
        this.ltsID = proto.getLtsid().toByteArray();
    }

    /**
     * Gets point X.
     */
    public Point getX() {
        return X;
    }

    /**
     * Gets the LTS ID.
     */
    public byte[] getLtsID() {
        return ltsID;
    }
}
