package ch.epfl.dedis.calypso;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.byzcoin.ByzCoinRPC;
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
import java.util.Collections;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ReadInstanceTest {
    private CalypsoRPC calypso;
    private WriteInstance w;
    private ReadInstance r;
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
        genesisDarc.addIdentity("spawn:calypsoRead", admin.getIdentity(), Rules.OR);

        calypso = new CalypsoRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(500, MILLIS));
        if (!calypso.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }

        String secret = "this is a secret";
        Document doc = new Document(secret.getBytes(), 16, null, genesisDarc.getBaseId());
        w = new WriteInstance(calypso, genesisDarc.getId(),
                Arrays.asList(admin), Collections.singletonList(1L),
                doc.getWriteData(calypso.getLTS()));
        assertTrue(calypso.getProof(w.getInstance().getId()).matches());

        r = new ReadInstance(calypso, w, Arrays.asList(admin), Collections.singletonList(2L));
        assertTrue(calypso.getProof(r.getInstance().getId()).matches());
    }

    @Test
    void testCopyReader() throws Exception {
        ReadInstance r2 = ReadInstance.fromByzCoin(calypso, r.getInstance().getId());
        assertTrue(calypso.getProof(r2.getInstance().getId()).matches());
    }

}