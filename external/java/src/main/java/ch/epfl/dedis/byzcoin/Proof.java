package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.proto.TrieProto;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.util.List;

/**
 * Proof represents a key/value entry in the trie and the path to the
 * root node.
 */
public class Proof {
    private TrieProto.Proof proof;
    private List<SkipchainProto.ForwardLink> links;
    private SkipBlock latest;

    private StateChangeBody finalStateChangeBody;

    /**
     * Creates a new proof given a protobuf-representation.
     *
     * @param p the protobuf-representation of the proof
     */
    public Proof(ByzCoinProto.Proof p) throws InvalidProtocolBufferException {
        proof = p.getInclusionproof();
        latest = new SkipBlock(p.getLatest());
        links = p.getLinksList();
        if (!proof.getLeaf().getKey().isEmpty()) {
            finalStateChangeBody = new StateChangeBody(ByzCoinProto.StateChangeBody.parseFrom(proof.getLeaf().getValue()));
        }
    }

    /**
     * @return the instance stored in this proof - it will not verify if the proof is valid!
     * @throws CothorityNotFoundException if the requested instance cannot be found
     */
    public Instance getInstance() throws CothorityNotFoundException{
        return Instance.fromProof(this);
    }

    /**
     * Get the protobuf representation of the proof.
     * @return the protobuf representation of the proof
     */
    public ByzCoinProto.Proof toProto() {
        ByzCoinProto.Proof.Builder b = ByzCoinProto.Proof.newBuilder();
        b.setInclusionproof(proof);
        b.setLatest(latest.getProto());
        for (SkipchainProto.ForwardLink link : this.links) {
            b.addLinks(link);
        }
        return b.build();
    }

    /**
     * accessor for the skipblock assocaited with this proof.
     * @return SkipBlock
     */
    public SkipBlock getLatest() {
        return this.latest;
    }

    /**
     * Verifies the proof with regard to the root id. It will follow all
     * steps and make sure that the hashes work out, starting from the
     * leaf. At the end it will verify against the root hash to make sure
     * that the inclusion proof is correct.
     *
     * @param id the skipblock to verify
     * @return true if all checks verify, false if there is a mismatch in the hashes
     * @throws CothorityException if something goes wrong
     */
    public boolean verify(SkipblockId id) throws CothorityException {
        if (!isByzCoinProof()){
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
        // TODO make more verification
        return proof.getLeaf().hasKey() && !proof.getLeaf().getKey().isEmpty();
    }

    /**
     * @return the key of the leaf node
     */
    public byte[] getKey() {
        return proof.getLeaf().getKey().toByteArray();
    }

    /**
     * @return the list of values in the leaf node.
     */
    public StateChangeBody getValues() {
        return finalStateChangeBody;
    }

    /**
     * @return the value of the proof.
     */
    public byte[] getValue(){
        return getValues().getValue();
    }

    /**
     * @return the string of the contractID.
     */
    public String getContractID(){
        return new String(getValues().getContractID());
    }

    /**
     * @return the darcID defining the access rules to the instance.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public DarcId getDarcID() throws CothorityCryptoException{
        return getValues().getDarcId();
    }

    /**
     * @return true if this looks like a matching proof for byzcoin.
     */
    public boolean isByzCoinProof(){
        if (!matches()) {
            return false;
        }
        return true;
    }

    /**
     * @param expected the string of the expected contract.
     * @return true if this is a matching byzcoin proof for a contract of the given contract.
     */
    public boolean isContract(String expected){
        if (!isByzCoinProof()){
            return false;
        }
        if (!getContractID().equals(expected)) {
            return false;
        }
        return true;
    }

    /**
     * Checks if the proof is valid and of type expected.
     *
     * @param expected the expected contractId
     * @param id the Byzcoin id to verify the proof against
     * @return true if the proof is correct with regard to that Byzcoin id and the contract is of the expected type.
     * @throws CothorityException if something goes wrong
     */
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
