package ch.epfl.dedis.omniledger;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.ClientTransaction;
import ch.epfl.dedis.lib.omniledger.Configuration;
import ch.epfl.dedis.lib.omniledger.Instruction;
import ch.epfl.dedis.lib.omniledger.OmniledgerRPC;
import ch.epfl.dedis.lib.omniledger.contracts.DarcInstance;
import ch.epfl.dedis.lib.omniledger.darc.Darc;
import ch.epfl.dedis.lib.omniledger.darc.Signer;
import ch.epfl.dedis.lib.omniledger.darc.SignerEd25519;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.Arrays;
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

        try {
            config = new Configuration(testInstanceController.getRoster(), Duration.of(100, MILLIS));
            ol = new OmniledgerRPC(genesisDarc, config);
            if (!ol.checkLiveness()){
                throw new CothorityCommunicationException("liveness check failed");
            }
        } catch (CothorityCommunicationException e) {
            logger.info("Error is: " + e.toString());
            logger.error("Couldn't start skipchain - perhaps you need to run the following commands:");
            logger.error("cd $(go env GOPATH)/src/github.com/dedis/onchain-secrets/conode");
            logger.error("./run_conode.sh local 4 2");
            fail("Couldn't start ocs!");
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
