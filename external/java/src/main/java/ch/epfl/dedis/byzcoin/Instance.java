package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;

import java.util.List;

/**
 * A contract represents the data that can be interpreted by the
 * corresponding contract.
 */
public class Instance {
    private InstanceId id;
    private String contractId;
    private DarcId darcId;
    private byte[] data;

    /**
     * Creates a new instance from its basic parameters.
     * @param id the id of the instance
     * @param cid the contractId, a string
     * @param did the darcId responsible for this instance
     * @param data the data stored in this instance
     */
    private Instance(InstanceId id, String cid, DarcId did, byte[] data){
        this.id = id;
        contractId = cid;
        darcId = did;
        this.data = data;
    }

    /**
     * Creates an instance from a proof received from ByzCoin.
     *
     * @param p the proof for the instance
     * @throws CothorityException
     */
    public static Instance fromProof(Proof p) throws CothorityNotFoundException {
        if (!p.matches()){
            throw new CothorityNotFoundException("this is a proof of absence");
        }
        List<byte[]> values = p.getValues();
        DarcId darcId;
        try {
            darcId = new DarcId(values.get(2));
        } catch (CothorityCryptoException e){
            throw new CothorityNotFoundException("couldn't get darc from proof: " + e.getMessage());
        }
        return new Instance(new InstanceId(p.getKey()), new String(values.get(1)), darcId, values.get(0));
    }

    /**
     * Creates an instance given an id and a Byzcoin service.
     *
     * @param bc a running Byzcoin service
     * @param id a valid instance id
     * @return
     * @throws CothorityCommunicationException
     * @throws CothorityNotFoundException
     */
    public static Instance fromByzcoin(ByzCoinRPC bc, InstanceId id) throws CothorityCommunicationException, CothorityNotFoundException{
        return fromProof(bc.getProof(id));
    }

    /**
     * @return the id of this instance.
     */
    public InstanceId getId() {
        return id;
    }

    /**
     * @return the contractId of this instance, which is a string.
     */
    public String getContractId() {
        return contractId;
    }

    /**
     * @return the darcid of this instance
     */
    public DarcId getDarcId() { return darcId; }

    /**
     * @return the data of this instance.
     */
    public byte[] getData() {
        return data;
    }
}