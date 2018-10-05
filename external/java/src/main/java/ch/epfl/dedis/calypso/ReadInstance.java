package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.Argument;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.byzcoin.transaction.Spawn;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Request;
import ch.epfl.dedis.lib.darc.Signature;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * ReadInstance represents a contract created by the calypsoRead contract.
 */
public class ReadInstance {
    public static String ContractId = "calypsoRead";
    private Instance instance;
    private CalypsoRPC calypso;
    private final static Logger logger = LoggerFactory.getLogger(ReadInstance.class);

    /**
     * Constructor used for when a new instance is needed.
     *
     * @param calypso The CalypsoRPC object.
     * @param write   The write instance where a new read instance should be spawned from.
     * @param signers Signers who are allowed to spawn a new instance.
     * @throws CothorityException
     */
    public ReadInstance(CalypsoRPC calypso, WriteInstance write, List<Signer> signers) throws CothorityException {
        this.calypso = calypso;
        ClientTransaction ctx = createCTX(write, signers);
        calypso.sendTransactionAndWait(ctx, 10);
        instance = getInstance(calypso, ctx.getInstructions().get(0).deriveId(""));
    }

    /**
     * Private constructor for already existing CalypsoRead instance.
     * @param calypso
     * @param inst
     */
    private ReadInstance(CalypsoRPC calypso, Instance inst){
        this.calypso = calypso;
        this.instance = inst;
    }

    /**
     * Get the instance object.
     */
    public Instance getInstance() {
        return instance;
    }

    /**
     * @return the readData stored in this instance.
     * @throws CothorityNotFoundException
     */
    public ReadData getRead() throws CothorityNotFoundException {
        return new ReadData(instance);
    }

    /**
     * Decrypts the key material by creating the read and write proofs necessary
     * for the LTS to allow a re-encryption to the public key stored in the
     * read request.
     * @param reader is the corresponding private key of the public
     * @return
     * @throws CothorityException
     */
    public byte[] decryptKeyMaterial(Scalar reader) throws CothorityException{
        Proof readProof = calypso.getProof(getInstance().getId());
        Proof writeProof = calypso.getProof(getRead().getWriteId());
        DecryptKeyReply dk = calypso.tryDecrypt(writeProof, readProof);
        return dk.getKeyMaterial(reader);
    }

    /**
     * Constructor used to connect to an existing instance.
     *
     * @param calypso The CalypsoRPC object.
     * @param id      The id of the instance.
     * @throws CothorityException
     */
    public static ReadInstance fromByzCoin(CalypsoRPC calypso, InstanceId id) throws CothorityException {
        return new ReadInstance(calypso, getInstance(calypso, id));
    }

    /**
     * Prepares a new transaction to create a read instance.
     *
     * @param consumers Array of Signers
     * @return
     * @throws CothorityCryptoException
     */
    private ClientTransaction createCTX(WriteInstance write, List<Signer> consumers) throws CothorityCryptoException {
        if (consumers.size() != 1) {
            throw new CothorityCryptoException("Currently only one signer supported.");
        }
        Signer consumer = consumers.get(0);
        Instance writeInstance = write.getInstance();

        // Create the Calypso.Read structure
        ReadData read = new ReadData(writeInstance.getId(), consumer.getPublic());
        List<Argument> args = new ArrayList<>();
        args.add(new Argument("read", read.toProto().toByteArray()));
        Spawn sp = new Spawn(ReadInstance.ContractId, args);
        Instruction inst = new Instruction(writeInstance.getId(), Instruction.genNonce(), 0, 1, sp);
        try {
            Request r = new Request(writeInstance.getDarcId(), "spawn:" + ReadInstance.ContractId, inst.hash(),
                    Arrays.asList(consumer.getIdentity()), null);
            logger.info("Signing: {}", Hex.printHexBinary(r.hash()));
            Signature sign = new Signature(consumer.sign(r.hash()), consumer.getIdentity());
            inst.setSignatures(Arrays.asList(sign));
        } catch (Signer.SignRequestRejectedException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
        return new ClientTransaction(Arrays.asList(inst));
    }

    /**
     * Create a spawn instruction with a read request and send it to the ledger.
     */
    private InstanceId read(ReadData rr, DarcId darcID, List<Signer> signers) throws CothorityException {
        Argument arg = new Argument("read", rr.toProto().toByteArray());

        Spawn spawn = new Spawn(ContractId, Arrays.asList(arg));
        Instruction instr = new Instruction(new InstanceId(darcID.getId()), Instruction.genNonce(), 0, 1, spawn);
        instr.signBy(darcID, signers);

        ClientTransaction tx = new ClientTransaction(Arrays.asList(instr));
        calypso.sendTransactionAndWait(tx, 5);

        return instr.deriveId("");
    }

    /**
     * Fetches the instance from ByzCoin over the network.
     *
     * @param id
     * @throws CothorityException
     */
    private static Instance getInstance(CalypsoRPC calypso, InstanceId id) throws CothorityException {
        Instance inst = calypso.getProof(id).getInstance();
        if (!inst.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", inst.getContractId());
            throw new CothorityNotFoundException("this is not an " + ContractId + " instance");
        }
        logger.info("new " + ContractId + " instance: " + inst.getId().toString());
        return inst;
    }
}
