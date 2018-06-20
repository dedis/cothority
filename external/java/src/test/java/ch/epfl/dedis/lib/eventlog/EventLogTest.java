package ch.epfl.dedis.lib.eventlog;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.omniledger.darc.Signer;
import ch.epfl.dedis.lib.omniledger.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class EventLogTest {

    private EventLog el;
    private long blockInterval;
    private TestServerController testInstanceController;

    @BeforeEach
    void testInit() throws CothorityCryptoException, CothorityCommunicationException {

        List<Signer> signers =  new ArrayList<>();
        signers.add(new SignerEd25519());
        testInstanceController = TestServerInit.getInstance();
        this.blockInterval = 2000000000; // 2 seconds
        this.el = new EventLog(testInstanceController.getRoster(), signers, this.blockInterval);
    }

    @Test
    void testLog() throws CothorityCryptoException, CothorityCommunicationException, InterruptedException {
        Event event = new Event("login", "alice");
        byte[] key = this.el.log(event);

        Thread.sleep(2 * this.blockInterval / 1000000);

        Event event2 = this.el.get(key);
        assertTrue(event.equals(event2));
    }

    @Test
    void testLogMore() throws CothorityCryptoException, CothorityCommunicationException, InterruptedException {
        int n = 50;
        List<byte[]> keys = new ArrayList<>(n);
        Event event = new Event("login", "alice");
        for (int i = 0; i < n; i++) {
            // The timestamps in these event are all the same, but doing el.log takes time and it may not be possible to
            // add all the events. So we have to limit the number of events that we add.
            keys.add(this.el.log(event));
        }
        Thread.sleep(2 * this.blockInterval / 1000000);
        for (byte[] key : keys) {
            Event event2 = this.el.get(key);
            assertTrue(event.equals(event2));
        }
    }

    @Test
    void testSearch() throws CothorityCryptoException, CothorityCommunicationException, InterruptedException {
        long now = System.currentTimeMillis() * 1000 * 1000;
        Event event = new Event(now, "login", "alice");
        this.el.log(event);

        Thread.sleep(2 * this.blockInterval / 1000000);

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