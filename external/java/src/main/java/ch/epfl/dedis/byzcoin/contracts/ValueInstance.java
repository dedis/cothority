package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.Argument;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.byzcoin.transaction.Invoke;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Identity;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;

/**
 * ValueInstance represents a simple value store on byzcoin.
 */
public class ValueInstance {
    public static String ContractId = "value";
    private Instance instance;
    private ByzCoinRPC bc;
    private byte[] value;

    private final static Logger logger = LoggerFactory.getLogger(ValueInstance.class);

    /**
     * Instantiates a new value in ByzCoin by sending a spawn instruction to a darc
     * that has a "spawn:value" rule in it.
     *
     * @param bc            a running ByzCoin service
     * @param spawnerDarcId a darc Id with a "spawn:value" rule in it
     * @param spawnerSigner a signer having the right to sign for the "spawn:value" rule
     * @param signerCtr     the monotonically increasing counters for the spawnSigner
     * @param value         the value to store in the instance
     * @throws CothorityException if something goes wrong
     */
    public ValueInstance(ByzCoinRPC bc, DarcId spawnerDarcId, Signer spawnerSigner, Long signerCtr, byte[] value) throws CothorityException {
        SecureDarcInstance spawner = SecureDarcInstance.fromByzCoin(bc, spawnerDarcId);
        List<Argument> args = new ArrayList<>();
        instance = spawner.spawnInstanceAndWait(ContractId, spawnerSigner, signerCtr, args, 10).getInstance();
        this.bc = bc;
        this.value = value;
    }

    /**
     * Instantiates a ValueInstance with the given parameters.
     *
     * @param bc       a running ByzCoin service
     * @param instance a value instance
     */
    private ValueInstance(ByzCoinRPC bc, Instance instance) throws CothorityNotFoundException {
        if (!instance.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a value instance");
        }
        this.bc = bc;
        this.instance = instance;
        this.value = instance.getData();
    }

    /**
     * Updates the value by getting the latest instance and updating it.
     *
     * @throws CothorityException if something goes wrong
     */
    public void update() throws CothorityException {
        instance = Instance.fromByzcoin(bc, instance.getId());
        value = instance.getData();
    }

    /**
     * Creates an instruction to evolve the value in byzcoin. The signer must have its identity in the current
     * darc as "invoke:update" rule.
     * <p>
     * TODO: allow for evolution if the expression has more than one identity.
     *
     * @param newValue  the value to replace the old value.
     * @param signerCtr the monotonically increasing counters which must map to the signers who will
     *                  eventually sign the instruction
     * @return Instruction to be sent to byzcoin
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Instruction evolveValueInstruction(byte[] newValue, Identity owner, Long signerCtr) {
        Invoke inv = new Invoke(ContractId, "update", ContractId, newValue);
        return new Instruction(instance.getId(),
                Collections.singletonList(owner),
                Collections.singletonList(signerCtr),
                inv);
    }

    /**
     * Asks byzcoin to update the value but does not wait for the transaction to be confirmed.
     *
     * @param newValue the value to replace the old value.
     * @param owner    is the owner that can sign to evolve the darc
     * @param ownerCtr the monotonically increasing counters for the owner
     * @throws CothorityException if something goes wrong
     */
    public void evolveValue(byte[] newValue, Signer owner, Long ownerCtr) throws CothorityException {
        Instruction inst = evolveValueInstruction(newValue, owner.getIdentity(), ownerCtr);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(Collections.singletonList(owner));
        bc.sendTransaction(ct);
    }

    /**
     * Asks byzcoin to update the value and waits until the new value has
     * been stored in the global state.
     * TODO: check if there has been an error in the transaction!
     *
     * @param newValue the value to replace the old value.
     * @param owner    is the owner that can sign to evolve the darc
     * @param ownerCtr the monotonically increasing counters for the owner
     * @param wait     how many blocks to wait for inclusion of the instruction
     * @throws CothorityException if something goes wrong
     */
    public void evolveValueAndWait(byte[] newValue, Signer owner, Long ownerCtr, int wait) throws CothorityException {
        Instruction inst = evolveValueInstruction(newValue, owner.getIdentity(), ownerCtr);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(Collections.singletonList(owner));
        bc.sendTransactionAndWait(ct, wait);
        value = newValue;
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
    public byte[] getValue() {
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

    /**
     * Instantiates a new ValueInstance given a working byzcoin service and
     * an instanceId. This instantiator will contact byzcoin and try to get
     * the current valueInstance. If the instance is not found, or is not of
     * contractId "Value", an exception will be thrown.
     *
     * @param bc is a running ByzCoin service
     * @param id of the value-instance to connect to
     * @return the new ValueInstance
     * @throws CothorityException if something goes wrong
     */
    public static ValueInstance fromByzcoin(ByzCoinRPC bc, InstanceId id) throws CothorityException {
        return new ValueInstance(bc, Instance.fromByzcoin(bc, id));
    }

    /**
     * Convenience function to connect to an existing ValueInstance.
     *
     * @param bc a running ByzCoin service
     * @param p  the proof for the valueInstance
     * @return the new ValueInstance
     * @throws CothorityException if something goes wrong
     */
    public static ValueInstance fromByzcoin(ByzCoinRPC bc, Proof p) throws CothorityException {
        return fromByzcoin(bc, new InstanceId(p.getKey()));
    }
}
