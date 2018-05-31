package ch.epfl.dedis.lib.eventlog;

import ch.epfl.dedis.lib.Local;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class EventLogTest {

    private EventLog el;
    private Signer signer;

    @BeforeEach
    void testInit() throws CothorityCryptoException, CothorityCommunicationException {
        this.signer = new SignerEd25519();
        this.el = new EventLog(Local.roster, signer);
    }

    @Test
    void testLog() {
        Event event = new Event("login", "alice");
        this.el.log(event);
    }
}