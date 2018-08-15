package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.contracts.DarcInstance;
import ch.epfl.dedis.lib.omniledger.contracts.ValueInstance;
import ch.epfl.dedis.lib.omniledger.darc.*;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.xml.bind.DatatypeConverter;
import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.*;

public class OmniledgerRPCTest {
    static OmniledgerRPC ol;

    static Signer admin;
    static Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(OmniledgerRPCTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        Rules rules = Darc.initRules(Arrays.asList(admin.getIdentity()),
                Arrays.asList(admin.getIdentity()));
        genesisDarc = new Darc(rules, "genesis".getBytes());

        ol = new OmniledgerRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(500, MILLIS));
        if (!ol.checkLiveness()){
            throw new CothorityCommunicationException("liveness check failed");
        }
    }

    @Test
    void ping() throws Exception{
        assertTrue(ol.checkLiveness());
    }

    @Test
    void updateDarc() throws Exception {
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        logger.info("DC is: {}", dc.getId());
        logger.info("genesisDarc is: {}", genesisDarc.getId());
        Darc newDarc = genesisDarc.copy();
        newDarc.setRule("spawn:darc", "all".getBytes());
        Instruction instr = dc.evolveDarcInstruction(newDarc, admin, 0, 1);
        logger.info("DC is: {}", dc.getId());
        ol.sendTransactionAndWait(new ClientTransaction(Arrays.asList(instr)), 10);

        dc.update();
        logger.info("darc-version is: {}", dc.getDarc().getVersion());
        assertEquals(newDarc.getVersion(), dc.getDarc().getVersion());
    }

    @Test
    void spawnDarc() throws Exception{
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        Darc darc2 = genesisDarc.copy();
        darc2.setRule("spawn:darc", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin);

        List<Identity> id = Arrays.asList(admin.getIdentity());
        Darc newDarc = new Darc(id, id, "new darc".getBytes());

        Proof p = dc.spawnContractAndWait(DarcInstance.ContractId, admin,
                Argument.NewList("darc", newDarc.toProto().toByteArray()), 10);
        assertTrue(p.matches());

        logger.info("creating DarcInstance");
        DarcInstance dc2 = new DarcInstance(ol, newDarc);
        logger.info("ids: {} - {}", dc2.getDarc().getId(), newDarc.getId());
        logger.info("ids: {} - {}", dc2.getDarc().getBaseId(), newDarc.getBaseId());
        logger.info("darcs:\n{}\n{}", dc2.getDarc(), newDarc);
        assertTrue(dc2.getDarc().getId().equals(newDarc.getId()));
    }

    @Test
    void spawnValue() throws Exception{
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        Darc darc2 = genesisDarc.copy();
        darc2.setRule("spawn:value", admin.getIdentity().toString().getBytes());
        darc2.setRule("invoke:update", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin);

        byte[] myvalue = "314159".getBytes();
        Proof p = dc.spawnContractAndWait(ValueInstance.ContractId, admin, Argument.NewList("value", myvalue), 10);
        assertTrue(p.matches());

        ValueInstance vi = new ValueInstance(ol, p);
        assertArrayEquals(vi.getValue(), myvalue);
        myvalue = "27".getBytes();
        vi.evolveValueAndWait(myvalue, admin);
        assertArrayEquals(vi.getValue(), myvalue);
    }

    @Test
    @Disabled
    void getLatestSkipblock() throws Exception{
        ol.update();
        SkipBlock previous = ol.getSkipchain().getLatestSkipblock();
        assertNotNull(previous);

        Thread.sleep(200);
        ol.update();
        SkipBlock latest = ol.getSkipchain().getLatestSkipblock();
        assertNotNull(latest);
        assertNotEquals(previous, latest);
        assertFalse(previous.getIndex() == latest.getIndex());
    }

    @Test
    void updateOL() throws Exception{
    }

    /**
     * We only give the client the roster and the genesis ID. It should be able to find the configuration, latest block
     * and the genesis darc.
     */
    @Test
    void reconnect() throws Exception {
        logger.info("Genesis darc is at {}", DatatypeConverter.printHexBinary(genesisDarc.getId().getId()));
        OmniledgerRPC ol2 = new OmniledgerRPC(ol.getRoster(), ol.getGenesis().getSkipchainId());
        assertEquals(ol.getConfig(), ol2.getConfig());
        assertEquals(ol.getSkipchain().getLatestSkipblock().getId(), ol2.getSkipchain().getLatestSkipblock().getId());
        assertEquals(ol.getGenesisDarc().getBaseId(), ol2.getGenesisDarc().getBaseId());
    }
}
