package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.SignerCounters;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Collections;

import static ch.epfl.dedis.byzcoin.ByzCoinRPCTest.BLOCK_INTERVAL;
import static org.junit.jupiter.api.Assertions.*;

public class NamingTest {
    private static ByzCoinRPC bc;
    private static Signer admin;
    private static Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(NamingTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());

        bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, BLOCK_INTERVAL);
        if (!bc.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }
    }

    /**
     * Name resolution tests only contains functional tests, more rigorous testing is in the go side.
     */
    @Test
    void nameResolution() throws Exception {
        SignerCounters counters = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));
        counters.increment();

        // add the _name rule
        Darc newGenesis = bc.getGenesisDarc().partialCopy();
        newGenesis.setRule("_name:darc", newGenesis.getExpression("spawn:darc"));
        logger.info("evolving darc");
        bc.getGenesisDarcInstance().evolveDarcAndWait(newGenesis, admin, counters.head(), 10);

        // create the naming instance
        counters.increment();
        logger.info("new naming");
        NamingInstance namingInst = new NamingInstance(bc, genesisDarc.getId(), Collections.singletonList(admin), counters.getCounters());

        // set a name for the genesis darc
        counters.increment();
        logger.info("name for genesis");
        namingInst.setAndWait("my genesis darc",
                new InstanceId(bc.getGenesisDarc().getBaseId().getId()),
                Collections.singletonList(admin),
                counters.getCounters(),
                10);

        // try to get the name back
        InstanceId iID = namingInst.resolve(bc.getGenesisDarc().getBaseId(), "my genesis darc");
        assertEquals(iID, new InstanceId(bc.getGenesisDarc().getBaseId().getId()));

        // set it again and it should fail
        counters.increment();
        logger.info("name again");
        assertThrows(CothorityException.class,
                () -> namingInst.setAndWait("my genesis darc",
                        new InstanceId(bc.getGenesisDarc().getBaseId().getId()),
                        Collections.singletonList(admin),
                        counters.getCounters(),
                        10));

        // remove the name
        // No need to increment because it failed previously)
        logger.info("remove name");
        namingInst.removeAndWait("my genesis darc",
                new InstanceId(bc.getGenesisDarc().getBaseId().getId()),
                Collections.singletonList(admin),
                counters.getCounters(),
                10);

        // remove the key again and it should fail
        counters.increment();
        logger.info("remove key");
        assertThrows(CothorityException.class, () -> namingInst.removeAndWait("my genesis darc",
                new InstanceId(bc.getGenesisDarc().getBaseId().getId()),
                Collections.singletonList(admin),
                counters.getCounters(),
                10));

        // try to get the name and it should fail
        logger.info("get name");
        assertThrows(CothorityCommunicationException.class,
                () -> namingInst.resolve(bc.getGenesisDarc().getBaseId(), "my genesis darc"));
    }

}
