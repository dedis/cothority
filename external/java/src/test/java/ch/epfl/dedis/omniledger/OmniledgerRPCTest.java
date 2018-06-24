package ch.epfl.dedis.omniledger;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.*;
import ch.epfl.dedis.lib.omniledger.contracts.DarcInstance;
import ch.epfl.dedis.lib.omniledger.contracts.ValueInstance;
import ch.epfl.dedis.lib.omniledger.darc.Darc;
import ch.epfl.dedis.lib.omniledger.darc.Identity;
import ch.epfl.dedis.lib.omniledger.darc.Signer;
import ch.epfl.dedis.lib.omniledger.darc.SignerEd25519;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

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
    static Configuration config;

    private final static Logger logger = LoggerFactory.getLogger(OmniledgerRPCTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        Map<String, byte[]> rules = Darc.initRules(Arrays.asList(admin.getIdentity()),
                Arrays.asList(admin.getIdentity()));
        genesisDarc = new Darc(rules, "genesis".getBytes());

        config = new Configuration(testInstanceController.getRoster(), Duration.of(100, MILLIS));
        ol = new OmniledgerRPC(genesisDarc, config);
        if (!ol.checkLiveness()){
            throw new CothorityCommunicationException("liveness check failed");
        }
    }

    @Test
    void ping() throws Exception{
        assertTrue(ol.checkLiveness());
    }

    @Test
    void updateDarc() throws Exception{
        SkipBlock previous = ol.getLatest();
        logger.info("Previous skipblock is: {}", previous.getIndex());
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        logger.info("DC is: {}", dc.getId());
        logger.info("genesisDarc is: {}", genesisDarc.getId());
        Darc newDarc = genesisDarc.copy();
        newDarc.setRule("spawn:darc", "all".getBytes());
        Instruction instr = dc.evolveDarcInstruction(newDarc, admin, 0, 1);
        logger.info("DC is: {}", dc.getId());
        ol.sendTransaction(new ClientTransaction(Arrays.asList(instr)));
        Thread.sleep(2000);
        ol.update();
        SkipBlock latest = ol.getLatest();
        logger.info("Previous skipblock is: {}", previous.getIndex());
        logger.info("Latest skipblock is: {}", latest.getIndex());
        assertFalse(previous.equals(latest));
        assertFalse(previous.getIndex() == latest.getIndex());

        dc.update();
        logger.info("darc-version is: {}", dc.getDarc().getVersion());
        assertEquals(dc.getDarc().getVersion(), newDarc.getVersion());

        dc.evolveDarcAndWait(newDarc, admin);
        logger.info("darc-version is: {}", dc.getDarc().getVersion());
        assertEquals(dc.getDarc().getVersion(), newDarc.getVersion());
    }

    @Test
    void spawnDarc() throws Exception{
        SkipBlock previous = ol.getLatest();
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        Darc darc2 = genesisDarc.copy();
        darc2.setRule("spawn:darc", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin);

        List<Identity> id = Arrays.asList(admin.getIdentity());
        Darc newDarc = new Darc(id, id, "new darc".getBytes());

        Proof p = dc.spawnContractAndWait("darc", admin,
                Argument.NewList("darc", newDarc.toProto().toByteArray()));
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
        SkipBlock previous = ol.getLatest();
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        Darc darc2 = genesisDarc.copy();
        darc2.setRule("spawn:value", admin.getIdentity().toString().getBytes());
        darc2.setRule("invoke:update", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin);

        byte[] myvalue = "314159".getBytes();
        Proof p = dc.spawnContractAndWait("value", admin, Argument.NewList("value", myvalue));
        assertTrue(p.matches());

        ValueInstance vi = new ValueInstance(ol, p);
        assertArrayEquals(vi.getValue(), myvalue);
        myvalue = "27".getBytes();
        vi.evolveValueAndWait(myvalue, admin);
        assertArrayEquals(vi.getValue(), myvalue);
    }

    @Test
    @Disabled
    void getLatest() throws Exception{
        ol.update();
        SkipBlock previous = ol.getLatest();
        assertNotNull(previous);

        Thread.sleep(200);
        ol.update();
        SkipBlock latest = ol.getLatest();
        assertNotNull(latest);
        assertFalse(previous.equals(latest));
        assertFalse(previous.getIndex() == latest.getIndex());
    }

    @Test
    void updateOL() throws Exception{
    }
}
