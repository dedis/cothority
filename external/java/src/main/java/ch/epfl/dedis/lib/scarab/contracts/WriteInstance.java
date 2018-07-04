package ch.epfl.dedis.lib.scarab.contracts;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.omniledger.*;
import ch.epfl.dedis.lib.omniledger.contracts.DarcInstance;
import ch.epfl.dedis.lib.omniledger.darc.Darc;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * WriteInstance holds the data related to a write request. It is a representation of what is
 * stored in OmniLedger. You can create it from an instanceID
 */
public class WriteInstance {
    private Instance instance;
    private OmniledgerRPC ol;

    private final static Logger logger = LoggerFactory.getLogger(WriteInstance.class);

    /**
     * Instantiates a new DarcInstance given a working omniledger instance and
     * an instanceId. This instantiator will contact omniledger and try to get
     * the current darcInstance. If the instance is not found, or is not of
     * contractId "darc", an exception will be thrown.
     *
     * @param ol is a link to an omniledger instance that is running
     * @param id of the darc-instance to connect to
     * @throws CothorityException
     */
    public WriteInstance(OmniledgerRPC ol, InstanceId id) throws CothorityException {
        this.ol = ol;
        Proof p = ol.getProof(id);
        instance = new Instance(p);
        if (!instance.getContractId().equals("darc")) {
            logger.error("wrong instance: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a darc instance");
        }
        try {
            darc = new Darc(instance.getData());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }

}
