package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.PointFactory;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.Calypso;

/**
 * LTS represents a Long Term Secret, which is the result of a DKG among multiple nodes in the cothority. Each node
 * has a share that he can use to re-encrypt a secret from Calypso.
 */
public class CreateLTSReply {
    private SkipblockId byzcoinId;
    private InstanceId instanceId;
    private Point X;

    /**
     * Creates a Long Term Secret from a LTSReply.
     *
     * @param reply the input reply
     */
    public CreateLTSReply(Calypso.CreateLTSReply reply) {
        this.byzcoinId = new SkipblockId(reply.getInstanceid());
        this.instanceId = new InstanceId(reply.getInstanceid());
        this.X = PointFactory.getInstance().fromProto(reply.getX());
    }

    /**
     * Getter for the ByzCoin ID
     */
    public SkipblockId getByzcoinId() {
        return byzcoinId;
    }

    /**
     * Getter for the Instance ID
     */
    public InstanceId getInstanceId() {
        return instanceId;
    }

    /**
     * Getter for the public key of the LTS
     */
    public Point getX() {
        return X;
    }

    public LTSId getLTSID() {
        try {
            return new LTSId(this.instanceId.getId());
        } catch (CothorityCryptoException e) {
            throw new RuntimeException(e.getMessage());
        }
    }
}
