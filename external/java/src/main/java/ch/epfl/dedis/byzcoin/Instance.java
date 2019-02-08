package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;

/**
 * A contract represents the data that can be interpreted by the
 * corresponding contract.
 */
public class Instance {
    private InstanceId id;
    private String contractId;
    private DarcId darcBaseID;
    private byte[] data;

    /**
     * Creates a new instance from its basic parameters.
     *
     * @param id   the id of the instance
     * @param cid  the contractId, a string
     * @param baseID  the Darc base ID responsible for this instance
     * @param data the data stored in this instance
     */
    private Instance(InstanceId id, String cid, DarcId baseID, byte[] data) {
        this.id = id;
        contractId = cid;
        darcBaseID = baseID;
        this.data = data;
    }

    /**
     * Creates an instance from a proof received from ByzCoin. This function expects the proof to be valid.
     *
     * @param p the proof for the instance
     * @return a new Instance
     */
    public static Instance fromProof(Proof p) {
        StateChangeBody body = p.getValues();
        return new Instance(new InstanceId(p.getKey()), new String(body.getContractID()), body.getDarcBaseId(), body.getValue());
    }

    /**
     * Creates an instance given an id and a Byzcoin service.
     *
     * @param bc a running Byzcoin service
     * @param id a valid instance id
     * @return a new Instance
     * @throws CothorityCommunicationException if something goes wrong
     * @throws CothorityNotFoundException      if the requested instance cannot be found
     * @throws CothorityCryptoException        if something is wrong with the proof
     */
    public static Instance fromByzcoin(ByzCoinRPC bc, InstanceId id) throws CothorityCommunicationException, CothorityNotFoundException, CothorityCryptoException {
        Proof p = bc.getProof(id);
        if (!p.exists(id.getId())) {
            throw new CothorityCryptoException("instance is not in proof");
        }
        return fromProof(p);
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
    public DarcId getDarcBaseID() {
        return darcBaseID;
    }

    /**
     * @return the data of this instance.
     */
    public byte[] getData() {
        return data;
    }
}