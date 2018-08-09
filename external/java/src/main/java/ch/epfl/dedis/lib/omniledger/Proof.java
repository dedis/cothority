package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import ch.epfl.dedis.proto.CollectionProto;
import ch.epfl.dedis.proto.OmniLedgerProto;
import com.google.protobuf.ByteString;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Proof represents a key/value entry in the collection and the path to the
 * root node.
 */
public class Proof {
    private OmniLedgerProto.Proof proof;
    private CollectionProto.Dump leaf;

    /**
     * Creates a new proof given a protobuf-representation.
     *
     * @param p
     */
    public Proof(OmniLedgerProto.Proof p) {
        proof = p;
        List<CollectionProto.Step> steps = p.getInclusionproof().getStepsList();
        CollectionProto.Dump left = steps.get(steps.size() - 1).getLeft();
        CollectionProto.Dump right = steps.get(steps.size() - 1).getRight();
        if (Arrays.equals(left.getKey().toByteArray(), getKey())) {
            leaf = left;
        } else if (Arrays.equals(right.getKey().toByteArray(), getKey())) {
            leaf = right;
        }
    }

    /**
     * Verifies the proof with regard to the root id. It will follow all
     * steps and make sure that the hashes work out, starting from the
     * leaf. At the end it will verify against the root hash to make sure
     * that the inclusion proof is correct.
     *
     * @return true if all checks verify, false if there is a mismatch in the
     * hashes
     * @throws CothorityException
     */
    public boolean verify() throws CothorityException {
        return false;
    }

    /**
     * @return true if the proof has the key/value pair stored, false if it
     * is a proof of absence.
     */
    public boolean matches() {
        return leaf != null;
    }

    /**
     * @return the key of the leaf node
     */
    public byte[] getKey() {
        return proof.getInclusionproof().getKey().toByteArray();
    }

    /**
     * @return the list of values in the leaf node.
     */
    public List<byte[]> getValues() {
        List<byte[]> ret = new ArrayList<>();
        for (ByteString v : leaf.getValuesList()) {
            ret.add(v.toByteArray());
        }
        return ret;
    }
}
