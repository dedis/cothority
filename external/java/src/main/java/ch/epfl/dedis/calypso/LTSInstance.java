package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.*;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.network.Roster;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

public class LTSInstance {
    public static String ContractId = "longTermSecret";
    public static String InvokeCommand = "reshare";
    private Instance instance;
    private CalypsoRPC calypso;
    private final static Logger logger = LoggerFactory.getLogger(LTSInstance.class);

    /**
     *
     * @param calypso
     * @param darcID
     * @param roster
     * @param signers
     * @param signerCtrs
     * @throws CothorityException
     */
    public LTSInstance(CalypsoRPC calypso, DarcId darcID, Roster roster, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        ClientTransaction ctx = createSpawnTx(new LTSInstanceInfo(roster), darcID, signers, signerCtrs);
        calypso.sendTransactionAndWait(ctx, 10);
        this.instance = getInstance(calypso, ctx.getInstructions().get(0).deriveId(""));
        this.calypso = calypso;
    }

    private LTSInstance(CalypsoRPC calypso, InstanceId instanceId) throws CothorityException  {
        Instance inst = getInstance(calypso, instanceId);
        this.calypso = calypso;
        this.instance = inst;
    }

    /**
     * Get the instance object.
     *
     * @return the Instance
     */
    public Instance getInstance() {
        return instance;
    }

    /**
     * Constructor used to connect to an existing instance.
     *
     * @param calypso the calypso RPC
     * @param id the instance ID to connect
     * @return an LTSInstance
     * @throws CothorityException if something goes wrong
     */
    public static LTSInstance fromByzCoin(CalypsoRPC calypso, InstanceId id) throws CothorityException {
        return new LTSInstance(calypso, id);
    }

    /**
     * Start the resharing process that re-distributes the LTS shares to a different roster.
     * Note that this is only the first step for re-sharing. After new roster is confirmed and stored in the block,
     * the client must instruct the CalypsoRPC to run the actual re-sharing protocol.
     * @param roster the new roster
     * @param signers the list of signers that are authorised to run the reshare contract
     * @param signerCtrs the monotonically increasing counter for every signer
     * @throws CothorityException if something goes wrong
     */
    public void reshareLTS(Roster roster, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        ClientTransaction ctx = createInvokeTx(new LTSInstanceInfo(roster), this.instance.getId(), signers, signerCtrs);
        this.calypso.sendTransactionAndWait(ctx, 10);
    }

    /**
     * Retrieves and verifies the proof for this instance.
     *
     * @return the proof
     * @throws CothorityCommunicationException when the communication fails
     * @throws CothorityCryptoException when the proof cannot be verified or the instance is not in the proof
     */
    public Proof getProofAndVerify() throws CothorityCommunicationException, CothorityCryptoException  {
        Proof p = this.calypso.getProof(this.instance.getId());
        if (!p.exists(this.instance.getId().getId())) {
            throw new CothorityCryptoException("instance is not in proof");
        }
        return p;
    }

    private static ClientTransaction createSpawnTx(LTSInstanceInfo info, DarcId darcID, List<Signer> signers, List<Long> signerCtrs) throws CothorityCryptoException {
        byte[] infoBuf = info.toProto().toByteArray();
        List<Argument> args = new ArrayList<>();
        args.add(new Argument("lts_instance_info", infoBuf));
        Spawn sp = new Spawn(LTSInstance.ContractId, args);
        Instruction inst = new Instruction(new InstanceId(darcID.getId()), signerCtrs, sp);
        ClientTransaction ctx = new ClientTransaction(Collections.singletonList(inst));
        ctx.signWith(signers);
        return ctx;
    }

    private static ClientTransaction createInvokeTx(LTSInstanceInfo info, InstanceId instanceId, List<Signer> signers, List<Long> signerCtrs) throws CothorityCryptoException {
        byte[] infoBuf = info.toProto().toByteArray();
        List<Argument> args = new ArrayList<>();
        args.add(new Argument("lts_instance_info", infoBuf));
        Invoke invoke = new Invoke(InvokeCommand, args);
        Instruction inst = new Instruction(instanceId, signerCtrs, invoke);
        ClientTransaction ctx = new ClientTransaction(Collections.singletonList(inst));
        ctx.signWith(signers);
        return ctx;
    }

    private static Instance getInstance(CalypsoRPC calypso, InstanceId id) throws CothorityException {
        Proof p = calypso.getProof(id);
        if (!p.exists(id.getId())) {
            throw new CothorityNotFoundException("instance is not in the proof");
        }
        Instance inst = p.getInstance();
        if (!inst.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", inst.getContractId());
            throw new CothorityNotFoundException("this is not an " + ContractId + " instance");
        }
        logger.info("new " + ContractId + " instance: " + inst.getId().toString());
        return inst;
    }
}
