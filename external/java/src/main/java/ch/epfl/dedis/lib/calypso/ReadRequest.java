package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.byzcoin.InstanceId;
import ch.epfl.dedis.proto.Calypso;

/**
 * A ReadRequest is the data that is sent to the calypsoRead contract in OmniLedger. It is used to log a read request
 * and must be linked to a corresponding write request.
 */
public class ReadRequest {
    private InstanceId writeId;
    private Point Xc;

    /**
     * Construct a read request given the ID of the corresponding write request and the reader's public key.
     */
    public ReadRequest(InstanceId writeId, Point readerPk) {
        this.writeId = writeId;
        this.Xc = readerPk;
    }

    /**
     * Return the protobuf representation of the ReadRequest.
     */
    public Calypso.Read toProto() {
        Calypso.Read.Builder b = Calypso.Read.newBuilder();
        b.setWrite(this.writeId.toByteString());
        b.setXc(this.Xc.toProto());
        return b.build();
    }
}
