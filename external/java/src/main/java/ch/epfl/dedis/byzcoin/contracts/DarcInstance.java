package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.*;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * DarcInstance represents a contract of a darc in ByzCoin. It is self-
 * sufficient, meaning it has a link to the byzcoin service it runs on.
 * If you evolve the DarcInstance, it will update its internal darc.
 */
public class DarcInstance {
    // ContractId is how the contract for a darc is represented.
    public static String ContractId = "darc";

    private Instance instance;
    private Darc darc;
    private ByzCoinRPC bc;

    private final static Logger logger = LoggerFactory.getLogger(DarcInstance.class);

    /**
     * Instantiates a new DarcInstance from an existing darc by sending a spawn instruction to
     * ByzCoin and then creating the instance from the existing darcInstance.
     *
     * @param bc            a running ByzCoin service
     * @param spawnerDarcId the darcId of a darc with the rights to spawn new darcs
     * @param spawnerSigner the signer with the rights to spawn new darcs
     * @param newDarc       the new darc to spawn
     * @throws CothorityException if something goes wrong
     */
    public DarcInstance(ByzCoinRPC bc, DarcId spawnerDarcId, Signer spawnerSigner, Darc newDarc) throws CothorityException {
        DarcInstance spawner = DarcInstance.fromByzCoin(bc, spawnerDarcId);
        DarcInstance newDarcInst = spawner.spawnDarcAndWait(newDarc, spawnerSigner, 10);
        this.bc = bc;
        darc = newDarc;
        instance = newDarcInst.getInstance();
    }

    /**
     * Instantiates a new DarcInstance given a byzcoin and
     * an instance. If the instance is not of type "darc", an exception will be thrown.
     *
     * @param bc   a running ByzCoin service
     * @param inst an instance representing a darc
     * @throws CothorityException if something goes wrong
     */
    private DarcInstance(ByzCoinRPC bc, Instance inst) throws CothorityException {
        this.bc = bc;
        if (!inst.getContractId().equals(ContractId)) {
            logger.error("wrong contract: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a darc contract");
        }
        instance = inst;
        try {
            darc = new Darc(instance.getData());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    /**
     * Update looks up the darc in ByzCoin and updates to the latest version, if available.
     *
     * @throws CothorityException if something goes wrong
     */
    public void update() throws CothorityException {
        instance = Instance.fromByzcoin(bc, instance.getId());
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
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Instruction evolveDarcInstruction(Darc newDarc, Signer owner, int pos, int len) throws CothorityCryptoException {
        newDarc.increaseVersion();
        newDarc.setPrevId(darc);
        newDarc.setBaseId(darc.getBaseId());
        if (!newDarc.getBaseId().equals(darc.getBaseId())) {
            throw new CothorityCryptoException("not darc with same baseID");
        }
        if (newDarc.getVersion() != darc.getVersion() + 1) {
            throw new CothorityCryptoException("not darc with next version");
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

    /**
     * Takes a new darc, increases its version, creates an instruction and sends it to ByzCoin, without
     * waiting an acknowledgement.
     *
     * @param newDarc the new darc, it should have the same version as the current darc
     * @param owner   a signer allowed to evolve the darc
     * @throws CothorityException if something goes wrong
     */
    public void evolveDarc(Darc newDarc, Signer owner) throws CothorityException {
        evolveDarcAndWait(newDarc, owner, 0);
    }

    /**
     * Asks byzcoin to evolve the darc and waits until the new darc has
     * been stored in the global state.
     * TODO: check if there has been an error in the transaction!
     *
     * @param newDarc is the new darc to replace the old one
     * @param owner   is the owner that can sign to evolve the darc
     * @param wait    the maximum number of blocks to wait
     * @return ClientTransactionId of the accepted transaction
     * @throws CothorityException if something goes wrong
     */
    public ClientTransactionId evolveDarcAndWait(Darc newDarc, Signer owner, int wait) throws CothorityException {
        Instruction inst = evolveDarcInstruction(newDarc, owner, 0, 1);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        return bc.sendTransactionAndWait(ct, wait);
    }

    /**
     * Creates an instruction for spawning an instance.
     * <p>
     * TODO: allow for multi-signatures
     *
     * @param contractID the id of the instance to create
     * @param s          the signer that is authorized to spawn this instance
     * @param args       arguments to give to the contract
     * @param pos        position in the ClientTransaction
     * @param len        total length of the ClientTransaction
     * @return the instruction to be added to the ClientTransaction
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Instruction spawnInstanceInstruction(String contractID, Signer s, List<Argument> args, int pos, int len)
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
     * Like spawnInstanceInstruction, but creates a ClientTransaction with only this instruction and sends it
     * to byzcoin.
     *
     * @param contractID the id of the instance to create
     * @param s          the signer that is authorized to spawn this contract-type
     * @param args       arguments to give to the contract
     * @throws CothorityException if something goes wrong
     * @return the client transaction ID
     */
    public ClientTransactionId spawnInstance(String contractID, Signer s, List<Argument> args) throws CothorityException {
        Instruction inst = spawnInstanceInstruction(contractID, s, args, 0, 1);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        return bc.sendTransaction(ct);
    }

    /**
     * Like spawnInstance but waits for the instance to be stored in byzcoin.
     *
     * @param contractID the id of the instance to create
     * @param s          the signer that is authorized to spawn this contract
     * @param args       arguments to give to the contract
     * @param wait       how many blocks to wait for the instance to be stored (0 = do not wait)
     * @return the Proof of inclusion
     * @throws CothorityException if something goes wrong
     */
    public Proof spawnInstanceAndWait(String contractID, Signer s, List<Argument> args, int wait) throws CothorityException {
        Instruction inst = spawnInstanceInstruction(contractID, s, args, 0, 1);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        bc.sendTransactionAndWait(ct, wait);
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
        return bc.getProof(iid);
    }

    /**
     * Spawns a new darc from the given darcInstance. The current darc needs to have the "spawn:darc"
     * rule with an expression that can be evaluated to true by the signer s.
     *
     * @param d    the new darc to store on ByzCoin
     * @param s    the signer allowed to spawn a new darc
     * @param wait how many blocks to wait. If it is 0, the call returns directly
     * @return a new DarcInstance, might be null if wait == 0
     * @throws CothorityException if something goes wrong if something went wrong
     */
    public DarcInstance spawnDarcAndWait(Darc d, Signer s, int wait) throws CothorityException {
        List<Argument> args = new ArrayList<>();
        args.add(new Argument("darc", d.toProto().toByteArray()));
        if (wait > 0) {
            Proof p = spawnInstanceAndWait(ContractId, s, args, wait);
            return new DarcInstance(this.bc, p.getInstance());
        } else {
            spawnInstance(ContractId, s, args);
            return null;
        }
    }

    /**
     * @return the id of the darc being held
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public DarcId getId() throws CothorityCryptoException {
        return darc.getId();
    }

    /**
     * @return a copy of the darc stored in this instance.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Darc getDarc() throws CothorityCryptoException {
        return darc.copyRulesAndVersion();
    }

    /**
     * @return the instance of the contract.
     */
    public Instance getInstance() {
        return instance;
    }

    /**
     * Instantiates a new DarcInstance given a working ByzCoin service and
     * an instanceId. This instantiator will contact byzcoin and try to get
     * the current darcInstance. If the instance is not found, or is not of
     * contractId "darc", an exception will be thrown.
     *
     * @param bc is a running ByzCoin ledger
     * @param id of the darc-instance to connect to
     * @return DarcInstance representing the latest version of the darc given in id
     * @throws CothorityException if something goes wrong
     */
    public static DarcInstance fromByzCoin(ByzCoinRPC bc, InstanceId id) throws CothorityException {
        return new DarcInstance(bc, Instance.fromByzcoin(bc, id));
    }

    /**
     * Instantiates a new DarcInstance given a working ByzCoin service and
     * an instanceId. This instantiator will contact byzcoin and try to get
     * the current darcInstance. If the instance is not found, or is not of
     * contractId "darc", an exception will be thrown.
     *
     * @param bc     is a running ByzCoin ledger that is running
     * @param baseId of the darc-instance to connect to
     * @return DarcInstance representing the latest version of the given baseId
     * @throws CothorityException if something goes wrong
     */
    public static DarcInstance fromByzCoin(ByzCoinRPC bc, DarcId baseId) throws CothorityException {
        return fromByzCoin(bc, new InstanceId(baseId.getId()));
    }

    /**
     * Instantiates a new DarcInstance given a working ByzCoin service and
     * an instanceId. This instantiator will contact byzcoin and try to get
     * the current darcInstance. If the instance is not found, or is not of
     * contractId "darc", an exception will be thrown.
     *
     * @param bc is a running ByzCoin ledger
     * @param d  of which the base id will be taken to search in ByzCoin
     * @return DarcInstance representing the latest version of the given baseId
     * @throws CothorityException if somethings goes wrong
     */
    public static DarcInstance fromByzCoin(ByzCoinRPC bc, Darc d) throws CothorityException {
        return fromByzCoin(bc, d.getBaseId());
    }
}
