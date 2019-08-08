package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.transaction.*;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.stream.Collectors;

/**
 * NamingInstance is represents an instance that can be used to give names to instance IDs.
 * This instance is a singleton on ByzCoin just like the config instance.
 */
public class NamingInstance {
    public static String ContractId = "naming";
    private Instance instance;
    private ByzCoinRPC bc;

    private final static Logger logger = LoggerFactory.getLogger(NamingInstance.class);

    /**
     * Create a naming instance. Note that the naming instance is a singleton.
     *
     * @param bc         is a running byzcoin service
     * @param darcID     is the darc that contains the "spawn:naming" rule, usually it is the genesis darc
     * @param signers    is a list of signers that holds the keys to the "_name" rule in the Darc the guards iID
     * @param signerCtrs is the list of monotonically increasing counters that will go into the instruction,
     *                   they must match the signers who will eventually sign the instruction
     * @throws CothorityException if the instance cannot be created
     */
    public NamingInstance(ByzCoinRPC bc, DarcId darcID, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        Instruction instr = makeSpawnInstruction(new InstanceId(darcID.getId()), signers, signerCtrs);
        ClientTransaction tx = new ClientTransaction(Collections.singletonList(instr), bc.getProtocolVersion());
        tx.signWith(signers);

        bc.sendTransactionAndWait(tx, 10);
        NamingInstance instance = NamingInstance.fromByzcoin(bc);
        this.bc = instance.bc;
        this.instance = instance.instance;
    }

    /**
     * Loads the singleton naming instance from ByzCoin.
     *
     * @param bc is a running ByzCoin service
     * @return a reference to the instance
     * @throws CothorityNotFoundException      if the instance cannot be found on ByzCoin, perhaps an old version that
     *                                         does not support naming is used
     * @throws CothorityCommunicationException if there is a communication error
     * @throws CothorityCryptoException        if result that we got back form ByzCoin contains a wrong proof
     */
    public static NamingInstance fromByzcoin(ByzCoinRPC bc) throws CothorityNotFoundException, CothorityCommunicationException, CothorityCryptoException {
        // The naming instance is a singleton, the 32 byte buffer that starts with a 1 holds the instance.
        byte[] iidBuf = new byte[32];
        iidBuf[0] = 1;
        return new NamingInstance(bc, Instance.fromByzcoin(bc, new InstanceId(iidBuf)));
    }

    /**
     * Asynchronously assigns a name to an instance ID. After the instance is named, ByzCoin.resolveInstanceID can be
     * used to resolve the name. Once set, the name or instance ID cannot be changed. It is not allowed to set a name
     * that was previously removed.
     *
     * @param instanceName is the name given to the instance ID
     * @param iID          is the to-be-named instance ID
     * @param signers      is a list of signers that holds the keys to the "_name" rule in the Darc the guards iID
     * @param signerCtrs   is the list of monotonically increasing counters that will go into the instruction,
     *                     they must match the signers who will eventually sign the instruction
     * @throws CothorityException if any error occurs
     */
    public void set(String instanceName, InstanceId iID, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        setAndWait(instanceName, iID, signers, signerCtrs, 0);
    }

    /**
     * Assigns a name to an instance ID and wait for confirmation. After the instance is named,
     * ByzCoin.resolveInstanceID can be used to resolve the name. Once set, the name or instance ID cannot be changed.
     * It is not allowed to set a name that was previously removed.
     *
     * @param instanceName is the name given to the instance ID
     * @param iID          is the to-be-named instance ID
     * @param signers      is a list of signers that holds the keys to the "_name" rule in the Darc the guards iID
     * @param signerCtrs   is the list of monotonically increasing counters that will go into the instruction,
     *                     they must match the signers who will eventually sign the instruction
     * @param wait         how many blocks to wait for inclusion of the transaction
     * @throws CothorityException if any error occurs
     */
    public void setAndWait(String instanceName, InstanceId iID, List<Signer> signers, List<Long> signerCtrs, int wait) throws CothorityException {
        Instruction instr = makeAddInstruction(instanceName, iID, signers, signerCtrs);
        ClientTransaction tx = new ClientTransaction(Collections.singletonList(instr), bc.getProtocolVersion());
        tx.signWith(signers);

        bc.sendTransactionAndWait(tx, wait);
    }

    /**
     * This method asynchronously removes an instance name. Once removed, the name cannot be used in set again.
     *
     * @param instanceName is the name of the instance to remove
     * @param iID          is instance ID which must have been named previously to instanceName
     * @param signers      is a list of signers that holds the keys to the "_name" rule in the Darc the guards iID
     * @param signerCtrs   is the list of monotonically increasing counters that will go into the instruction,
     *                     they must match the signers who will eventually sign the instruction
     * @throws CothorityException
     */
    public void remove(String instanceName, InstanceId iID, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        removeAndWait(instanceName, iID, signers, signerCtrs, 0);
    }

    /**
     * This method removes an instance name and then waits for confirmation. Once removed, the name cannot be used in
     * set again.
     *
     * @param instanceName is the name of the instance to remove
     * @param iID          is instance ID which must have been named previously to instanceName
     * @param signers      is a list of signers that holds the keys to the "_name" rule in the Darc the guards iID
     * @param signerCtrs   is the list of monotonically increasing counters that will go into the instruction,
     *                     they must match the signers who will eventually sign the instruction
     * @param wait         how many blocks to wait for inclusion of the transaction
     * @throws CothorityException
     */
    public void removeAndWait(String instanceName, InstanceId iID, List<Signer> signers, List<Long> signerCtrs, int wait) throws CothorityException {
        Instruction instr = makeRemoveInstruction(instanceName, iID, signers, signerCtrs);
        ClientTransaction tx = new ClientTransaction(Collections.singletonList(instr), bc.getProtocolVersion());
        tx.signWith(signers);

        bc.sendTransactionAndWait(tx, wait);
    }

    /**
     * Resolves a previously named instance ID from a darc ID and a name.
     *
     * @param dID  is the darc ID that guards the instance.
     * @param name is the name given to the instance when it was named.
     * @return the instance ID.
     * @throws CothorityCommunicationException if the name does not exist or other failures.
     */
    public InstanceId resolve(DarcId dID, String name) throws CothorityCommunicationException {
        return bc.resolveInstanceID(dID, name);
    }

    private NamingInstance(ByzCoinRPC bc, Instance instance) throws CothorityNotFoundException {
        if (!instance.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a value instance");
        }
        this.bc = bc;
        this.instance = instance;
    }

    private Instruction makeInstruction(String instanceName, InstanceId iID, List<Signer> owners, List<Long> ownerCtrs, String cmd) {
        List<Argument> args = new ArrayList<>();
        args.add(new Argument("name", instanceName.getBytes()));
        args.add(new Argument("instanceID", iID.getId()));
        Invoke inv = new Invoke(ContractId, cmd, args);

        return new Instruction(
                this.instance.getId(),
                owners.stream().map(Signer::getIdentity).collect(Collectors.toList()),
                ownerCtrs,
                inv);
    }

    private Instruction makeRemoveInstruction(String instanceName, InstanceId iID, List<Signer> owners, List<Long> ownerCtrs) {
        return makeInstruction(instanceName, iID, owners, ownerCtrs, "remove");
    }

    private Instruction makeAddInstruction(String instanceName, InstanceId iID, List<Signer> owners, List<Long> ownerCtrs) {
        return makeInstruction(instanceName, iID, owners, ownerCtrs, "add");
    }

    private Instruction makeSpawnInstruction(InstanceId iID, List<Signer> owners, List<Long> ownerCtrs) {
        Spawn spawn = new Spawn(ContractId, new ArrayList<>());

        return new Instruction(
                iID,
                owners.stream().map(Signer::getIdentity).collect(Collectors.toList()),
                ownerCtrs,
                spawn);
    }
}
