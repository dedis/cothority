package ch.epfl.dedis.calypso;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Rules;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.Arrays;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.assertTrue;

class WriteInstanceTest {
    private CalypsoRPC calypso;
    private WriteInstance w;
    private Signer admin;
    private Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(WriteInstanceTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());
        genesisDarc.addIdentity("spawn:calypsoWrite", admin.getIdentity(), Rules.OR);

        calypso = new CalypsoRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(500, MILLIS));
        if (!calypso.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }

        String secret = "this is a secret";
        Document doc = new Document(secret.getBytes(), 16, null, genesisDarc.getBaseId());
        w = new WriteInstance(calypso, genesisDarc.getId(), Arrays.asList(admin), doc.getWriteData(calypso.getLTS()));

        Proof p = calypso.getProof(w.getInstance().getId());
        assertTrue(p.matches());
    }

    @Test
    void testCopyWriter() throws Exception {
        WriteInstance w2 = WriteInstance.fromCalypso(calypso, w.getInstance().getId());
        assertTrue(calypso.getProof(w2.getInstance().getId()).matches());
    }
}