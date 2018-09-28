package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.transaction.Argument;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.byzcoin.transaction.Spawn;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;
import java.util.List;

/**
 * WriteInstance holds the data related to a spawnCalypsoWrite request. It is a representation of what is
 * stored in ByzCoin. You can create it from an instanceID
 */
public class WriteInstance {
    public static String ContractId = "calypsoWrite";
    private Instance instance;
    private CalypsoRPC calypso;
    private LTS lts;

    private final static Logger logger = LoggerFactory.getLogger(WriteInstance.class);

    /**
     * Constructor for creating a new instance.
     *
     * @param calypso The CalypsoRPC object which should be already running.
     * @param darcId  The darc ID for which the signers belong.
     * @param signers The list of signers that are authorised to create the instance.
     * @param wr      The WriteData object, to be stored in the instance.
     * @throws CothorityException
     */
    public WriteInstance(CalypsoRPC calypso, DarcId darcId, List<Signer> signers, WriteData wr) throws CothorityException {
        this.calypso = calypso;
        this.lts = calypso.getLTS();
        InstanceId id = spawnCalypsoWrite(wr, darcId, signers);
        instance = getInstance(id);
    }

    /**
     * Constructor to connect to an existing instance.
     *
     * @param calypso The ByzCoinRPC object which should be already running.
     * @param id      The ID of the instance to connect.
     * @throws CothorityException
     */
    private WriteInstance(CalypsoRPC calypso, InstanceId id) throws CothorityException {
        this.calypso = calypso;
        instance = getInstance(id);
        lts = calypso.getLTS();
    }

    /**
     * Get the LTS configuration.
     */
    public LTS getLts() {
        return lts;
    }

    /**
     * Get the instance.
     */
    public Instance getInstance() {
        return instance;
    }

    public DarcId getDarcId() {
        return instance.getDarcId();
    }

    /**
     * @return the WriteData stored in that instance
     * @throws InvalidProtocolBufferException if the instance data is corrupt
     * @throws CothorityNotFoundException     if the instance does not hold a CalypsoWrite data
     */
    public WriteData getWrite() throws CothorityNotFoundException {
        return WriteData.fromInstance(getInstance());
    }

    /**
     * Spawns a new CalypsoRead instance for this Write Instance.
     *
     * @param calypso an existing calypso object
     * @param readers one or more readers that can sign the read spawn instruction
     * @return ReadInstance if successful
     * @throws CothorityException
     */
    public ReadInstance spawnCalypsoRead(CalypsoRPC calypso, List<Signer> readers) throws CothorityException {
        return new ReadInstance(calypso, this, readers);
    }

    /**
     * Fetches an already existing writeInstance from Calypso and returns it.
     *
     * @param calypso
     * @param writeId
     * @return
     * @throws CothorityException
     */
    public static WriteInstance fromCalypso(CalypsoRPC calypso, InstanceId writeId) throws CothorityException {
        return new WriteInstance(calypso, writeId);
    }

    /**
     * Create a spawn instruction with a spawnCalypsoWrite request and send it to the ledger.
     */
    private InstanceId spawnCalypsoWrite(WriteData req, DarcId darcID, List<Signer> signers) throws CothorityException {
        Argument arg = new Argument("write", req.getWrite().toByteArray());
        Spawn spawn = new Spawn(ContractId, Arrays.asList(arg));
        Instruction instr = new Instruction(new InstanceId(darcID.getId()), Instruction.genNonce(), 0, 1, spawn);
        instr.signBy(darcID, signers);

        ClientTransaction tx = new ClientTransaction(Arrays.asList(instr));
        calypso.sendTransactionAndWait(tx, 5);

        return instr.deriveId("");
    }

    // TODO same as what's in EventLogInstance, make a super class?
    private Instance getInstance(InstanceId id) throws CothorityException {
        Instance inst = calypso.getProof(id).getInstance();
        if (!inst.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", inst.getContractId());
            throw new CothorityNotFoundException("this is not an " + ContractId + " instance");
        }
        return inst;
    }
}
