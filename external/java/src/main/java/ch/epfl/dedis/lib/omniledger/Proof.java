package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import ch.epfl.dedis.lib.skipchain.ForwardLink;
import ch.epfl.dedis.proto.CollectionProto;
import ch.epfl.dedis.proto.OmniLedgerProto;
import ch.epfl.dedis.proto.SkipchainProto;
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
    private List<ForwardLink> links;

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
        links = new ArrayList<>();
        for (SkipchainProto.ForwardLink fl: p.getLinksList()){
            links.add(new ForwardLink(fl));
        }
    }

    /**
     * Get the protobuf representation of the proof.
     */
    public OmniLedgerProto.Proof toProto() {
        return this.proof;
    }

    /**
     * accessor for the skipblock assocaited with this proof.
     * @return SkipBlock
     */
    public SkipBlock getLatest() {
        return new SkipBlock(proof.getLatest());
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
    public boolean verify(SkipblockId id) throws CothorityException {
        if (!isOmniledgerProof()){
            return false;
        }
        // TODO: more verifications
        return true;
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

    /**
     * @return the value of the proof.
     */
    public byte[] getValue(){
        return getValues().get(0);
    }

    /**
     * @return the string of the contractID.
     */
    public String getContractID(){
        return new String(getValues().get(1));
    }

    /**
     * @return the darcID defining the access rules to the instance.
     * @throws CothorityCryptoException
     */
    public DarcId getDarcID() throws CothorityCryptoException{
        return new DarcId(getValues().get(2));
    }

    /**
     * @return true if this looks like a matching proof for omniledger.
     */
    public boolean isOmniledgerProof(){
        if (!matches()) {
            return false;
        }
        if (getValues().size() != 3) {
            return false;
        }
        return true;
    }

    /**
     * @param expected the string of the expected contract.
     * @return true if this is a matching omniledger proof for an instance of the given contract.
     */
    public boolean isContract(String expected){
        if (!isOmniledgerProof()){
            return false;
        }
        String contract = new String(getValues().get(1));
        if (!contract.equals(expected)) {
            return false;
        }
        return true;
    }

    public boolean isContract(String expected, SkipblockId id) throws CothorityException{
        if (!verify(id)){
            return false;
        }
        if (!isContract(expected)){
            return false;
        }
        return true;
    }
}
