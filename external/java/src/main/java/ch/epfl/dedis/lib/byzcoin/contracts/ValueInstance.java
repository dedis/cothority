package ch.epfl.dedis.lib.byzcoin.contracts;

import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.byzcoin.*;
import ch.epfl.dedis.lib.byzcoin.darc.Request;
import ch.epfl.dedis.lib.byzcoin.darc.Signature;
import ch.epfl.dedis.lib.byzcoin.darc.Signer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;

/**
 * ValueInstance represents a simple value store on byzcoin.
 */
public class ValueInstance {
    // ContractId is how a valueInstance is represented in OmniLedger.
    public static String ContractId = "value";
    private Instance instance;
    private ByzCoinRPC ol;
    private byte[] value;

    private final static Logger logger = LoggerFactory.getLogger(ValueInstance.class);

    /**
     * Instantiates a new ValueInstance given a working byzcoin instance and
     * an instanceId. This instantiator will contact byzcoin and try to get
     * the current valueInstance. If the instance is not found, or is not of
     * contractId "Value", an exception will be thrown.
     *
     * @param ol is a link to an byzcoin instance that is running
     * @param id of the value-instance to connect to
     * @throws CothorityException
     */
    public ValueInstance(ByzCoinRPC ol, InstanceId id) throws CothorityException {
        this.ol = ol;
        Proof p = ol.getProof(id);
        instance = new Instance(p);
        if (!instance.getContractId().equals(ContractId)) {
            logger.error("wrong instance: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a value instance");
        }
        value = instance.getData();
    }

    public ValueInstance(ByzCoinRPC ol, Proof p) throws CothorityException {
        this(ol, new InstanceId(p.getKey()));
    }

    public void update() throws CothorityException {
        instance = new Instance(ol.getProof(instance.getId()));
        value = instance.getData();
    }

    /**
     * Creates an instruction to evolve the value in byzcoin. The signer must have its identity in the current
     * darc as "invoke:update" rule.
     * <p>
     * TODO: allow for evolution if the expression has more than one identity.
     *
     * @param newValue the value to replace the old value.
     * @param owner    must have its identity in the "invoke:update" rule
     * @param pos      position of the instruction in the ClientTransaction
     * @param len      total number of instructions in the ClientTransaction
     * @return Instruction to be sent to byzcoin
     * @throws CothorityCryptoException
     */
    public Instruction evolveValueInstruction(byte[] newValue, Signer owner, int pos, int len) throws CothorityCryptoException {
        Invoke inv = new Invoke("update", ContractId, newValue);
        Instruction inst = new Instruction(instance.getId(), Instruction.genNonce(), pos, len, inv);
        try {
            Request r = new Request(instance.getDarcId(), "invoke:update", inst.hash(),
                    Arrays.asList(owner.getIdentity()), null);
            logger.info("Signing: {}", Hex.printHexBinary(r.hash()));
            Signature sign = new Signature(owner.sign(r.hash()), owner.getIdentity());
            inst.setSignatures(Arrays.asList(sign));
        } catch (Signer.SignRequestRejectedException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
        return inst;
    }

    public void evolveValue(byte[] newValue, Signer owner) throws CothorityException {
        Instruction inst = evolveValueInstruction(newValue, owner, 0, 1);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ol.sendTransaction(ct);
    }

    /**
     * Asks byzcoin to update the value and waits until the new value has
     * been stored in the global state.
     * TODO: check if there has been an error in the transaction!
     *
     * @param newValue the value to replace the old value.
     * @param owner   is the owner that can sign to evolve the darc
     * @throws CothorityException
     */
    public void evolveValueAndWait(byte[] newValue, Signer owner) throws CothorityException {
        evolveValue(newValue, owner);
        for (int i = 0; i < 10; i++) {
            Proof p = ol.getProof(instance.getId());
            Instance inst = new Instance(p);
            logger.info("Values are: {} - {}", Hex.printHexBinary(inst.getData()),
                    Hex.printHexBinary(newValue));
            if (Arrays.equals(inst.getData(), newValue)){
                value = newValue;
                return;
            }
            try{
                Thread.sleep(ol.getConfig().getBlockInterval().toMillis());
            } catch (InterruptedException e) {
                throw new RuntimeException(e);
            }
        }
        throw new CothorityCommunicationException("couldn't find new value");
    }

    /**
     * @return the id of the instance
     */
    public InstanceId getId() {
        return instance.getId();
    }

    /**
     * @return a copy of the value stored in this instance.
     */
    public byte[] getValue() throws CothorityCryptoException {
        byte[] v = new byte[value.length];
        System.arraycopy(value, 0, v, 0, value.length);
        return v;
    }

    /**
     * @return the instance used.
     */
    public Instance getInstance() {
        return instance;
    }
}
