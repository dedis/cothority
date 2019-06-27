package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.transaction.Argument;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.byzcoin.transaction.Invoke;
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

public class NamingInstance {
    public static String ContractId = "naming";
    private Instance instance;
    private ByzCoinRPC bc;

    private final static Logger logger = LoggerFactory.getLogger(NamingInstance.class);

    private NamingInstance(ByzCoinRPC bc, Instance instance) throws CothorityNotFoundException {
        if (!instance.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a value instance");
        }
        this.bc = bc;
        this.instance = instance;
    }

    public static NamingInstance fromByzcoin(ByzCoinRPC bc) throws CothorityNotFoundException, CothorityCommunicationException, CothorityCryptoException {
        // The naming instance is a singleton, the 32 byte buffer that starts with a 1 holds the instance.
        byte[] iidBuf  = new byte[32];
        iidBuf[0] = 1;
        return new NamingInstance(bc, Instance.fromByzcoin(bc, new InstanceId(iidBuf)));
    }

    public void setInstanceName(String instanceName, InstanceId iID, List<Signer> owners, List<Long> ownerCtrs) throws CothorityException {
        bc.sendTransaction(makeNamingTx(instanceName, iID, owners, ownerCtrs));
    }

    public void setInstanceNameAndWait(String instanceName, InstanceId iID, List<Signer> owners, List<Long> ownerCtrs, int wait) throws CothorityException {
        bc.sendTransactionAndWait(makeNamingTx(instanceName, iID, owners, ownerCtrs), wait);
    }

    private ClientTransaction makeNamingTx(String instanceName, InstanceId iID, List<Signer> owners, List<Long> ownerCtrs) throws CothorityCryptoException {
        List<Argument> args = new ArrayList<>();
        args.add(new Argument("name", instanceName.getBytes()));
        args.add(new Argument("instanceID", iID.getId()));
        Invoke inv = new Invoke(ContractId, "add", args);
        Instruction namingInst = new Instruction(
                this.instance.getId(),
                owners.stream().map(Signer::getIdentity).collect(Collectors.toList()),
                ownerCtrs,
                inv);
        ClientTransaction ct = new ClientTransaction(Collections.singletonList(namingInst));
        ct.signWith(owners);
        return ct;
    }

}
