package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.*;
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
import java.util.Collections;
import java.util.List;

/**
 * DarcInstance represents a contract of a darc in ByzCoin. It is self-
 * sufficient, meaning it has a link to the byzcoin service it runs on.
 * If you evolve the DarcInstance, it will update its internal darc.
 */
public class SecureDarcInstance {
    // ContractId is how the contract for a darc is represented.
    public static String ContractId = "secure_darc";

    private Instance instance;
    private Darc darc;
    private ByzCoinRPC bc;

    private final static Logger logger = LoggerFactory.getLogger(SecureDarcInstance.class);

    /**
     * Instantiates a new DarcInstance from an existing darc by sending a spawn instruction to
     * ByzCoin and then creating the instance from the existing darcInstance.
     *
     * @param bc            a running ByzCoin service
     * @param spawnerDarcId the darcId of a darc with the rights to spawn new darcs
     * @param spawnerSigner the signer with the rights to spawn new darcs
     * @param signerCtr     the monotonically increasing counters for the spawnSigner
     * @param newDarc       the new darc to spawn
     * @throws CothorityException if something goes wrong
     */
    public SecureDarcInstance(ByzCoinRPC bc, DarcId spawnerDarcId, Signer spawnerSigner, Long signerCtr, Darc newDarc) throws CothorityException {
        SecureDarcInstance spawner = SecureDarcInstance.fromByzCoin(bc, spawnerDarcId);
        SecureDarcInstance newDarcInst = spawner.spawnDarcAndWait(newDarc, spawnerSigner, signerCtr, 10);
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
    private SecureDarcInstance(ByzCoinRPC bc, Instance inst) throws CothorityException {
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
     * darc as "invoke:secure_darc:evolve" rule.
     * TODO: allow for evolution if the expression has more than one identity.
     *
     * @param newDarc   the darc to replace the old darc, the version, prevID and baseID attributes are ignored and set
     *                  automatically by this function
     * @param signerCtr is the monotonically increasing counter which must match the signer who will eventually
     *                  sign the returned instruction.
     * @return Instruction to be sent to byzcoin
     */
    public Instruction evolveDarcInstruction(Darc newDarc, Long signerCtr) {
        return evolveDarcInstruction(newDarc, signerCtr, false);
    }

    /**
     * Creates an instruction to (unrestricted) evolve the darc in byzcoin. The signer must have its identity in the current
     * darc as "invoke:secure_darc.evolve" or "invoke:secure_darc.evolve_unrestricted" rule, depending on the unrestricted flag.
     * TODO: allow for evolution if the expression has more than one identity.
     *
     * @param newDarc      the darc to replace the old darc, the version, prevID and baseID attributes are ignored and set
     *                     automatically by this function
     * @param signerCtr    is the monotonically increasing counter which must match the signer who will eventually
     *                     sign the returned instruction.
     * @param unrestricted whether to use the unrestricted evolution
     * @return Instruction to be sent to byzcoin
     */
    public Instruction evolveDarcInstruction(Darc newDarc, Long signerCtr, boolean unrestricted) {
        newDarc.setVersion(this.getDarc().getVersion() + 1);
        newDarc.setPrevId(darc);
        newDarc.setBaseId(darc.getBaseId());
        String cmd = "evolve";
        if (unrestricted) {
            cmd = "evolve_unrestricted";
        }
        Invoke inv = new Invoke(ContractId, cmd, "darc", newDarc.toProto().toByteArray());
        byte[] d = newDarc.getBaseId().getId();
        return new Instruction(new InstanceId(d), Collections.singletonList(signerCtr), inv);
    }

    /**
     * Takes a new darc, increases its version, creates an instruction and sends it to ByzCoin, without
     * waiting an acknowledgement.
     *
     * @param newDarc  the new darc, the version, prevID and baseID attributes are ignored and set
     *                 automatically by this function
     * @param owner    a signer allowed to evolve the darc
     * @param ownerCtr a monotonically increasing counter which must map to the owners
     * @throws CothorityException if something goes wrong
     */
    public void evolveDarc(Darc newDarc, Signer owner, Long ownerCtr) throws CothorityException {
        evolveDarcAndWait(newDarc, owner, ownerCtr, 0);
    }

    /**
     * Asks byzcoin to evolve the darc and waits until the new darc has
     * been stored in the global state.
     *
     * @param newDarc  the darc to replace the old darc, the version, prevID and baseID attributes are ignored and set
     *                 automatically by this function
     * @param owner    is the owner that can sign to evolve the darc
     * @param ownerCtr a monotonically increasing counter which must map to the owners
     * @param wait     the maximum number of blocks to wait
     * @return ClientTransactionId of the accepted transaction
     * @throws CothorityException if something goes wrong
     */
    public ClientTransactionId evolveDarcAndWait(Darc newDarc, Signer owner, Long ownerCtr, int wait) throws CothorityException {
        Instruction inst = evolveDarcInstruction(newDarc, ownerCtr);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(Collections.singletonList(owner));
        return bc.sendTransactionAndWait(ct, wait);
    }

    /**
     * Asks byzcoin to (unrestricted) evolve the darc and waits until the new darc has
     * been stored in the global state.
     *
     * @param newDarc      the darc to replace the old darc, the version, prevID and baseID attributes are ignored and set
     *                     automatically by this function
     * @param owner        is the owner that can sign to evolve the darc
     * @param ownerCtr     a monotonically increasing counter which must map to the owners
     * @param wait         the maximum number of blocks to wait
     * @param unrestricted whether to use the unrestricted evolution
     * @return ClientTransactionId of the accepted transaction
     * @throws CothorityException if something goes wrong
     */
    public ClientTransactionId evolveDarcAndWait(Darc newDarc, Signer owner, Long ownerCtr, int wait, boolean unrestricted) throws CothorityException {
        Instruction inst = evolveDarcInstruction(newDarc, ownerCtr, unrestricted);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(Collections.singletonList(owner));
        return bc.sendTransactionAndWait(ct, wait);
    }

    /**
     * Creates an instruction for spawning an instance.
     * <p>
     * TODO: allow for multi-signatures
     *
     * @param contractID the id of the instance to create
     * @param signerCtr  the next counter which the signer should use
     * @param args       arguments to give to the contract
     * @return the instruction to be added to the ClientTransaction
     */
    public Instruction spawnInstanceInstruction(String contractID, Long signerCtr, List<Argument> args) {
        Spawn sp = new Spawn(contractID, args);
        return new Instruction(new InstanceId(darc.getBaseId().getId()), Collections.singletonList(signerCtr), sp);
    }

    /**
     * Like spawnInstanceInstruction, but creates a ClientTransaction with only this instruction and sends it
     * to byzcoin.
     *
     * @param contractID the id of the instance to create
     * @param s          the signer that is authorized to spawn this contract-type
     * @param signerCtr  a monotonically increasing counter which must map to the signer s
     * @param args       arguments to give to the contract
     * @return the client transaction ID
     * @throws CothorityException if something goes wrong
     */
    public ClientTransactionId spawnInstance(String contractID, Signer s, Long signerCtr, List<Argument> args) throws CothorityException {
        Instruction inst = spawnInstanceInstruction(contractID, signerCtr, args);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(Collections.singletonList(s));
        return bc.sendTransaction(ct);
    }

    /**
     * Like spawnInstance but waits for the instance to be stored in byzcoin.
     *
     * @param contractID the id of the instance to create
     * @param s          the signer that is authorized to spawn this contract
     * @param signerCtr  a monotonically increasing counter which must map to the signer s
     * @param args       arguments to give to the contract
     * @param wait       how many blocks to wait for the instance to be stored (0 = do not wait)
     * @return the Proof of inclusion
     * @throws CothorityException if something goes wrong
     */
    public Proof spawnInstanceAndWait(String contractID, Signer s, Long signerCtr, List<Argument> args, int wait) throws CothorityException {
        Instruction inst = spawnInstanceInstruction(contractID, signerCtr, args);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(Collections.singletonList(s));

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
        Proof p = bc.getProof(iid);
        if (!p.exists(iid.getId())) {
            throw new CothorityCryptoException("instance is not in proof");
        }
        return p;
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
    public SecureDarcInstance spawnDarcAndWait(Darc d, Signer s, Long signerCounter, int wait) throws CothorityException {
        List<Argument> args = new ArrayList<>();
        args.add(new Argument("darc", d.toProto().toByteArray()));
        if (wait > 0) {
            Proof p = spawnInstanceAndWait(ContractId, s, signerCounter, args, wait);
            return new SecureDarcInstance(this.bc, p.getInstance());
        } else {
            spawnInstance(ContractId, s, signerCounter, args);
            return null;
        }
    }

    /**
     * @return the id of the darc being held
     */
    public DarcId getId() {
        return darc.getId();
    }

    /**
     * @return the darc of this instance.
     */
    public Darc getDarc() {
        return darc;
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
    public static SecureDarcInstance fromByzCoin(ByzCoinRPC bc, InstanceId id) throws CothorityException {
        return new SecureDarcInstance(bc, Instance.fromByzcoin(bc, id));
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
    public static SecureDarcInstance fromByzCoin(ByzCoinRPC bc, DarcId baseId) throws CothorityException {
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
    public static SecureDarcInstance fromByzCoin(ByzCoinRPC bc, Darc d) throws CothorityException {
        return fromByzCoin(bc, d.getBaseId());
    }
}
