package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.lib.byzcoin.*;
import ch.epfl.dedis.lib.byzcoin.darc.DarcId;
import ch.epfl.dedis.lib.byzcoin.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;
import java.util.List;

/**
 * WriteInstance holds the data related to a write request. It is a representation of what is
 * stored in ByzCoin. You can create it from an instanceID
 */
public class WriterInstance {
    private static String ContractId = "calypsoWrite";
    private Instance instance;
    private ByzCoinRPC bc;
    private CreateLTSReply ltsData;

    private final static Logger logger = LoggerFactory.getLogger(WriterInstance.class);

    /**
     * Constructor for creating a new instance.
     *
     * @param bc      The ByzCoinRPC object which should be already running.
     * @param signers The list of signers that are authorised to create the instance.
     * @param darcId  The darc ID for which the signers belong.
     * @param ltsData The LTS data, must be created via the Calypso RPC call if it does not exist yet.
     * @param wr      The WriteRequest object, to be stored in the instance.
     * @throws CothorityException
     */
    public WriterInstance(ByzCoinRPC bc, List<Signer> signers, DarcId darcId, CreateLTSReply ltsData, WriteRequest wr) throws CothorityException {
        this.bc = bc;
        this.ltsData = ltsData;
        InstanceId id = this.write(wr, darcId, signers);
        this.setInstance(id);
    }

    /**
     * Constructor to connect to an existing instance.
     *
     * @param bc      The ByzCoinRPC object which should be already running.
     * @param id      The ID of the instance to connect.
     * @param ltsData The LTS configuration.
     * @throws CothorityException
     */
    public WriterInstance(ByzCoinRPC bc, InstanceId id, CreateLTSReply ltsData) throws CothorityException {
        this.bc = bc;
        this.setInstance(id);
        this.ltsData = new CreateLTSReply(ltsData);
    }

    /**
     * Get the LTS configuration.
     */
    public CreateLTSReply getLtsData() {
        return ltsData;
    }

    /**
     * Get the instance.
     */
    public Instance getInstance() {
        return instance;
    }

    /**
     * Create a spawn instruction with a write request and send it to the ledger.
     */
    private InstanceId write(WriteRequest req, DarcId darcID, List<Signer> signers) throws CothorityException {
        Argument arg = new Argument("write", req.toProto(this.ltsData.getX(), this.ltsData.getLtsID()).toByteArray());

        Spawn spawn = new Spawn(ContractId, Arrays.asList(arg));
        Instruction instr = new Instruction(new InstanceId(darcID.getId()), Instruction.genNonce(), 0, 1, spawn);
        instr.signBy(darcID, signers);

        ClientTransaction tx = new ClientTransaction(Arrays.asList(instr));
        bc.sendTransactionAndWait(tx, 5);

        return instr.deriveId("");
    }

    // TODO same as what's in EventLogInstance, make a super class?
    private void setInstance(InstanceId id) throws CothorityException {
        Proof p = bc.getProof(id);
        Instance inst = new Instance(p);
        if (!inst.getContractId().equals(ContractId)) {
            logger.error("wrong instance: {}", inst.getContractId());
            throw new CothorityNotFoundException("this is not an " + ContractId + " instance");
        }
        this.instance = inst;
        logger.info("new " + ContractId + " instance: " + inst.getId().toString());
    }
}
