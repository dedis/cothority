package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.SignerCounters;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Rules;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.eventlog.Event;
import ch.epfl.dedis.eventlog.EventLogInstance;
import ch.epfl.dedis.eventlog.SearchResponse;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class EventlogTest {
    private static ByzCoinRPC bc;
    private static EventLogInstance el;
    private static Signer admin;
    private static Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(EventlogTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());
        genesisDarc.addIdentity("spawn:eventlog", admin.getIdentity(), Rules.OR);
        genesisDarc.addIdentity("invoke:eventlog", admin.getIdentity(), Rules.OR);

        bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(500, MILLIS));
        if (!bc.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }

        // Get the counter for the admin
        SignerCounters adminCtrs = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        el = new EventLogInstance(bc, genesisDarc.getId(), Arrays.asList(admin), Collections.singletonList(adminCtrs.head()+1));
    }

    @Test
    void log() throws Exception {
        // Get the counter for the admin
        SignerCounters adminCtrs = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));
        adminCtrs.increment();

        Event e = new Event("hello", "goodbye");
        InstanceId key = el.log(e, Arrays.asList(admin), adminCtrs.getCounters());
        Thread.sleep(5 * bc.getConfig().getBlockInterval().toMillis());
        Event loggedEvent = el.get(key);
        assertEquals(loggedEvent, e);
    }

    @Test
    void logMore() throws Exception {
        // Get the counter for the admin
        SignerCounters adminCtrs = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        int n = 50;
        List<InstanceId> keys = new ArrayList<>(n);
        Event event = new Event("login", "alice");
        for (int i = 0; i < n; i++) {
            // The timestamps in these event are all the same, but doing el.log takes time and it may not be possible to
            // add all the events. So we have to limit the number of events that we add.
            adminCtrs.increment();
            keys.add(el.log(event, Arrays.asList(admin), adminCtrs.getCounters()));
        }
        boolean allOK = true;
        for (int i = 0; i < 4; i++) {
            allOK = true;
            Thread.sleep(5 * bc.getConfig().getBlockInterval().toMillis());
            for (InstanceId key : keys) {
                try {
                    logger.info("ok");
                    // this checks the trie proofs.
                    Event event2 = el.get(key);
                    assertEquals(event, event2);
                } catch (CothorityCryptoException e){
                    logger.info("bad");
                    allOK = false;
                    break;
                }
            }
            if (allOK){
                break;
            }
        }
        assertTrue(allOK, "one of the events failed");

        // check that we can't get an event that doesn't exist
        InstanceId badKey = new InstanceId(Hex.parseHexBinary("CDC4FB0BDD74CD86410DC80C818E7A0DB3C6452C9161CF7C6FC16D00C5CF0DA7"));
        assertThrows(CothorityCryptoException.class, () -> el.get(badKey));

        // Try to reconnect after doing a lot of transactions.
        ByzCoinRPC.fromByzCoin(bc.getRoster(), bc.getGenesisBlock().getId());
    }

    @Test
    void search() throws Exception {
        // Get the counter for the admin
        SignerCounters adminCtrs = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));
        adminCtrs.increment();

        long now = System.currentTimeMillis() * 1000 * 1000;
        Event event = new Event(now, "login", "alice");
        el.log(event, Arrays.asList(admin), adminCtrs.getCounters());

        Thread.sleep(5 * bc.getConfig().getBlockInterval().toMillis());

        // finds the event under any topic
        SearchResponse resp = el.search("", now - 1000, now + 1000);
        assertEquals(1, resp.events.size());
        assertEquals(resp.events.get(0), event);
        assertTrue(!resp.truncated);

        // finds the event under the right topic
        resp = el.search("login", now - 1000, now + 1000);
        assertEquals(1, resp.events.size());
        assertEquals(resp.events.get(0), event);
        assertTrue(!resp.truncated);

        // event does not exist
        resp = el.search("", now - 2000, now - 1000);
        assertEquals(0, resp.events.size());
    }
}
