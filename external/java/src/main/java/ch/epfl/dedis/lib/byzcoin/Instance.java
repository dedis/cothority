package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.lib.byzcoin.darc.DarcId;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;

import java.util.List;

/**
 * An instance represents the data that can be interpreted by the
 * corresponding contract.
 */
public class Instance {
    private InstanceId id;
    private String contractId;
    private DarcId darcId;
    private byte[] data;

    /**
     * Instantiates an instance given a proof sent by byzcoin.
     * @param p
     * @throws CothorityException
     */
    public Instance(Proof p) throws CothorityException {
        if (!p.matches()){
            throw new CothorityNotFoundException("this is a proof of absence");
        }
        id = new InstanceId(p.getKey());
        List<byte[]> values = p.getValues();
        data = values.get(0);
        contractId = new String(values.get(1));
        darcId = new DarcId(values.get(2));
    }

    /**
     * @return the instanceid of this instance.
     */
    public InstanceId getId() {
        return id;
    }

    /**
     * @return the contractid of this instance, which is a string.
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