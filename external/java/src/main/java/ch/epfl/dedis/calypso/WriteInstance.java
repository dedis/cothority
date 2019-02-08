package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.Argument;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.byzcoin.transaction.Spawn;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

/**
 * WriteInstance holds the data related to a spawnCalypsoWrite request. It is a representation of what is
 * stored in ByzCoin. You can create it from an instanceID
 */
public class WriteInstance {
    public static String ContractId = "calypsoWrite";
    private Instance instance;
    private CalypsoRPC calypso;
    private CreateLTSReply lts;

    private final static Logger logger = LoggerFactory.getLogger(WriteInstance.class);

    /**
     * Constructor for creating a new instance.
     *
     * @param calypso    The CalypsoRPC object which should be already running.
     * @param darcBaseID     The darc ID for which the signers belong.
     * @param signers    The list of signers that are authorised to create the instance.
     * @param signerCtrs a list of monotonically increasing counter for every signer
     * @param wr         The WriteData object, to be stored in the instance.
     * @throws CothorityException if something goes wrong
     */
    public WriteInstance(CalypsoRPC calypso, DarcId darcBaseID, List<Signer> signers, List<Long> signerCtrs, WriteData wr) throws CothorityException {
        this.calypso = calypso;
        this.lts = calypso.getLTS();
        InstanceId id = spawnCalypsoWrite(wr, darcBaseID, signers, signerCtrs);
        instance = getInstance(id);
    }

    /**
     * Constructor to connect to an existing instance.
     *
     * @param calypso The ByzCoinRPC object which should be already running.
     * @param id      The ID of the instance to connect.
     * @throws CothorityException if something goes wrong
     */
    private WriteInstance(CalypsoRPC calypso, InstanceId id) throws CothorityException {
        this.calypso = calypso;
        instance = getInstance(id);
        lts = calypso.getLTS();
    }

    /**
     * Get the LTS configuration.
     *
     * @return the LTS
     */
    public CreateLTSReply getLts() {
        return lts;
    }

    /**
     * Get the instance.
     *
     * @return the Instance
     */
    public Instance getInstance() {
        return instance;
    }

    public DarcId getDarcId() {
        return instance.getDarcBaseID();
    }

    /**
     * @return the WriteData stored in that instance
     * @throws CothorityNotFoundException if the instance does not hold a CalypsoWrite data
     */
    public WriteData getWrite() throws CothorityNotFoundException {
        return WriteData.fromInstance(getInstance());
    }

    /**
     * Spawns a new CalypsoRead instance for this Write Instance.
     *
     * @param calypso    an existing calypso object
     * @param readers    one or more readers that can sign the read spawn instruction
     * @param readerCtrs a list of monotonically increasing counter for every reader
     * @param Xc         is the key to which the dataEnc will be re-encrypted to, it must not be one of the signers
     * @return ReadInstance if successful
     * @throws CothorityException if something goes wrong
     */
    public ReadInstance spawnCalypsoRead(CalypsoRPC calypso, List<Signer> readers, List<Long> readerCtrs, Point Xc) throws CothorityException {
        return new ReadInstance(calypso, this, readers, readerCtrs, Xc);
    }

    /**
     * Fetches an already existing writeInstance from Calypso and returns it.
     *
     * @param calypso the Calypso instance
     * @param writeId the write instance to load
     * @return the new WriteInstance
     * @throws CothorityException if something goes wrong
     */
    public static WriteInstance fromCalypso(CalypsoRPC calypso, InstanceId writeId) throws CothorityException {
        return new WriteInstance(calypso, writeId);
    }

    /**
     * Create a spawn instruction with a spawnCalypsoWrite request and send it to the ledger.
     */
    private InstanceId spawnCalypsoWrite(WriteData req, DarcId darcBaseID, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        Argument arg = new Argument("write", req.toProto().toByteArray());
        Spawn spawn = new Spawn(ContractId, Arrays.asList(arg));
        Instruction instr = new Instruction(new InstanceId(darcBaseID.getId()),
                signers.stream().map(Signer::getIdentity).collect(Collectors.toList()),
                signerCtrs,
                spawn);

        ClientTransaction tx = new ClientTransaction(Arrays.asList(instr));
        tx.signWith(signers);
        calypso.sendTransactionAndWait(tx, 5);

        return instr.deriveId("");
    }

    // TODO same as what's in EventLogInstance, make a super class?
    private Instance getInstance(InstanceId id) throws CothorityException {
        Proof p = calypso.getProof(id);
        if (!p.exists(id.getId())) {
            throw new CothorityNotFoundException("instance is not in the proof");
        }
        Instance inst = p.getInstance();
        if (!inst.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", inst.getContractId());
            throw new CothorityNotFoundException("this is not an " + ContractId + " instance");
        }
        return inst;
    }
}
