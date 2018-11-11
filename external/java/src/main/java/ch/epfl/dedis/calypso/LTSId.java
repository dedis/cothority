package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.Sha256id;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

/**
 * This class represents a LTSId that points to a Long Term Secret configuration in Calypso.
 */
public class LTSId extends Sha256id {
    public LTSId(byte[] id) throws CothorityCryptoException{
        super(id);
    }

    public LTSId(ByteString bs) throws CothorityCryptoException{
        this(bs.toByteArray());
    }
}
