package ch.epfl.dedis.lib.eventlog;

import ch.epfl.dedis.lib.Local;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;

class EventLogTest {

    private EventLog el;

    @BeforeEach
    void testInit() throws CothorityCryptoException, CothorityCommunicationException {
        List<Signer> signers =  new ArrayList<>();
        signers.add(new SignerEd25519());
        this.el = new EventLog(Local.roster, signers);
    }

    @Test
    void testLog() throws CothorityCryptoException, CothorityCommunicationException, InterruptedException {
        Event event = new Event("login", "alice");
        byte[] key = this.el.log(event);

        Thread.sleep(10 * 1000);

        Event event2 = this.el.get(key);
        assertTrue(event.equals(event2));
    }

    @Test
    void testLogMore() throws CothorityCryptoException, CothorityCommunicationException, InterruptedException {
        int n = 100;
        Event event = new Event("login", "alice");
        List<byte[]> keys = new ArrayList<>(n);
        for (int i = 0; i < n; i++) {
            keys.add(this.el.log(event));
        }
        Thread.sleep(10 * 1000);
        for (byte[] key : keys) {
            Event event2 = this.el.get(key);
            assertTrue(event.equals(event2));
        }
    }
}