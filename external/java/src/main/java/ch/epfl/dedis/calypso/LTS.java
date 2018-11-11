package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.Calypso;

/**
 * LTS represents a Long Term Secret, which is the result of a DKG among multiple nodes in the cothority. Each node
 * has a share that he can use to re-encrypt a secret from Calypso.
 */
public class LTS {
    private LTSId ltsId;
    private Point X;

    /**
     * Creates a Long Term Secret from a LTSReply.
     * @param reply the input reply
     */
    public LTS(Calypso.CreateLTSReply reply){
        try {
            ltsId = new LTSId(reply.getLtsid());
        } catch (CothorityCryptoException e){
            throw new RuntimeException(e.getMessage());
        }
        X = new Ed25519Point(reply.getX());
    }

    /**
     * Creates a long term secret from an id and a point.
     * @param id the id
     * @param X the point
     */
    public LTS(LTSId id, Point X){
        this.ltsId = id;
        this.X = X;
    }

    /**
     * @return the Id of the Long Term Secret group
     */
    public LTSId getLtsId() {
        return ltsId;
    }

    /**
     * @return the shared public key of the Long Term Secret group
     */
    public Point getX() {
        return X;
    }
}
