package ch.epfl.dedis.lib.byzcoin.contracts;

import ch.epfl.dedis.lib.byzcoin.*;
import ch.epfl.dedis.lib.byzcoin.darc.*;
import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;
import java.util.List;

/**
 * DarcInstance represents an instance of a darc on Omniledger. It is self-
 * sufficient, meaning it has a link to the byzcoin instance it runs on.
 * If you evolve the DarcInstance, it will update its internal darc.
 */
public class DarcInstance {
    // ContractId is how the contract for a darc is represented.
    public static String ContractId = "darc";

    private Instance instance;
    private Darc darc;
    private ByzCoinRPC ol;

    private final static Logger logger = LoggerFactory.getLogger(DarcInstance.class);

    /**
     * Instantiates a new DarcInstance given a working byzcoin instance and
     * an instanceId. This instantiator will contact byzcoin and try to get
     * the current darcInstance. If the instance is not found, or is not of
     * contractId "darc", an exception will be thrown.
     *
     * @param ol is a link to an byzcoin instance that is running
     * @param id of the darc-instance to connect to
     * @throws CothorityException
     */
    public DarcInstance(ByzCoinRPC ol, InstanceId id) throws CothorityException {
        this.ol = ol;
        Proof p = ol.getProof(id);
        instance = new Instance(p);
        if (!instance.getContractId().equals(ContractId)) {
            logger.error("wrong instance: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a darc instance");
        }
        try {
            darc = new Darc(instance.getData());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    public DarcInstance(ByzCoinRPC ol, Darc d) throws CothorityException {
        this(ol, new InstanceId(d.getBaseId().getId()));
    }

    public void update() throws CothorityException {
        instance = new Instance(ol.getProof(instance.getId()));
        try {
            darc = new Darc(instance.getData());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    /**
     * Creates an instruction to evolve the darc in byzcoin. The signer must have its identity in the current
     * darc as "Invoke_Evolve" rule.
     * <p>
     * TODO: allow for evolution if the expression has more than one identity.
     *
     * @param newDarc the darc to replace the old darc.
     * @param owner   must have its identity in the "Invoke_Evolve" rule
     * @param pos     position of the instruction in the ClientTransaction
     * @param len     total number of instructions in the ClientTransaction
     * @return Instruction to be sent to byzcoin
     * @throws CothorityCryptoException
     */
    public Instruction evolveDarcInstruction(Darc newDarc, Signer owner, int pos, int len) throws CothorityCryptoException {
        newDarc.increaseVersion();
        newDarc.setPrevId(darc);
        newDarc.setBaseId(darc.getBaseId());
        if (!newDarc.getBaseId().equals(darc.getBaseId()) ||
                newDarc.getVersion() != darc.getVersion() + 1) {
            throw new CothorityCryptoException("not correct darc to evolve");
        }
        Invoke inv = new Invoke("evolve", ContractId, newDarc.toProto().toByteArray());
        byte[] d = newDarc.getBaseId().getId();
        Instruction inst = new Instruction(new InstanceId(d), Instruction.genNonce(), pos, len, inv);
        try {
            Request r = new Request(darc.getBaseId(), "invoke:evolve", inst.hash(),
                    Arrays.asList(owner.getIdentity()), null);
            logger.info("Signing: {}", Hex.printHexBinary(r.hash()));
            Signature sign = new Signature(owner.sign(r.hash()), owner.getIdentity());
            inst.setSignatures(Arrays.asList(sign));
        } catch (Signer.SignRequestRejectedException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
        return inst;
    }

    public void evolveDarc(Darc newDarc, Signer owner) throws CothorityException {
        Instruction inst = evolveDarcInstruction(newDarc, owner, 0, 1);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ol.sendTransaction(ct);
    }

    /**
     * Asks byzcoin to evolve the darc and waits until the new darc has
     * been stored in the global state.
     * TODO: check if there has been an error in the transaction!
     *
     * @param newDarc is the new darc to replace the old one
     * @param owner   is the owner that can sign to evolve the darc
     * @throws CothorityException
     */
    public void evolveDarcAndWait(Darc newDarc, Signer owner) throws CothorityException {
        evolveDarc(newDarc, owner);
        for (int i = 0; i < 10; i++) {
            Proof p = ol.getProof(instance.getId());
            Instance inst = new Instance(p);
            try {
                darc = new Darc(inst.getData());
                if (darc.getVersion() == newDarc.getVersion()) {
                    return;
                }
                Thread.sleep(ol.getConfig().getBlockInterval().toMillis());
            } catch (InvalidProtocolBufferException e) {
                continue;
            } catch (InterruptedException e) {
                throw new RuntimeException(e);
            }
        }
        throw new CothorityCommunicationException("didn't find new darc");
    }

    /**
     * Creates an instruction for spawning a contract.
     * <p>
     * TODO: allow for multi-signatures
     *
     * @param contractID the id of the contract to create
     * @param s          the signer that is authorized to spawn this contract
     * @param args       arguments to give to the contract
     * @param pos        position in the ClientTransaction
     * @param len        total length of the ClientTransaction
     * @return the instruction to be added to the ClientTransaction
     * @throws CothorityCryptoException
     */
    public Instruction spawnContractInstruction(String contractID, Signer s, List<Argument> args, int pos, int len)
            throws CothorityCryptoException {
        Spawn sp = new Spawn(contractID, args);
        Instruction inst = new Instruction(new InstanceId(darc.getBaseId().getId()), Instruction.genNonce(), pos, len, sp);
        try {
            Request r = new Request(darc.getBaseId(), "spawn:" + contractID, inst.hash(),
                    Arrays.asList(s.getIdentity()), null);
            logger.info("Signing: {}", Hex.printHexBinary(r.hash()));
            Signature sign = new Signature(s.sign(r.hash()), s.getIdentity());
            inst.setSignatures(Arrays.asList(sign));
        } catch (Signer.SignRequestRejectedException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
        return inst;
    }

    /**
     * Like spawnContractInstruction, but creates a ClientTransaction with only this instruction and sends it
     * to the byzcoin.
     *
     * @param contractID the id of the contract to create
     * @param s          the signer that is authorized to spawn this contract
     * @param args       arguments to give to the contract
     * @throws CothorityException
     */
    public ClientTransactionId spawnContract(String contractID, Signer s, List<Argument> args) throws CothorityException {
        Instruction inst = spawnContractInstruction(contractID, s, args, 0, 1);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        return ol.sendTransaction(ct);
    }

    /**
     * Like spawnContract but waits for the instance to be stored in byzcoin.
     *
     * @param contractID the id of the contract to create
     * @param s          the signer that is authorized to spawn this contract
     * @param args       arguments to give to the contract
     * @throws CothorityException
     */
    public Proof spawnContractAndWait(String contractID, Signer s, List<Argument> args, int wait) throws CothorityException {
        Instruction inst = spawnContractInstruction(contractID, s, args, 0, 1);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ol.sendTransactionAndWait(ct, wait);
        InstanceId iid = inst.deriveId("");
        if (contractID.equals(ContractId)) {
            // Special case for a darc, then the resulting instanceId is based
            // on the darc itself.
            try {
                Darc d = new Darc(args.get(0).getValue());
                iid = new InstanceId(d.getBaseId().getId());
            } catch (InvalidProtocolBufferException e) {
                throw new CothorityCommunicationException("this is not a correct darc-spawn");
            }
        }
        logger.info("waiting on iid {}", iid);
        return ol.getProof(iid);
    }

    /**
     * @return the id of the darc being held
     * @throws CothorityCryptoException
     */
    public DarcId getId() throws CothorityCryptoException {
        return darc.getId();
    }

    /**
     * @return a copy of the darc stored in this instance.
     */
    public Darc getDarc() throws CothorityCryptoException {
        return darc.copy();
    }

    /**
     * @return the instance used.
     */
    public Instance getInstance() {
        return instance;
    }
}
