package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.lib.byzcoin.Proof;
import ch.epfl.dedis.lib.byzcoin.darc.Darc;
import ch.epfl.dedis.lib.byzcoin.darc.Rules;
import ch.epfl.dedis.lib.byzcoin.darc.Signer;
import ch.epfl.dedis.lib.byzcoin.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.Arrays;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.assertTrue;

class WriterInstanceTest {
    private ByzCoinRPC bc;
    private WriterInstance w;
    private Signer admin;
    private Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(WriterInstanceTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        Rules rules = Darc.initRules(Arrays.asList(admin.getIdentity()),
                Arrays.asList(admin.getIdentity()));
        rules.addRule("spawn:calypsoWrite", admin.getIdentity().toString().getBytes());
        genesisDarc = new Darc(rules, "genesis".getBytes());

        bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(500, MILLIS));
        if (!bc.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }

        CreateLTSReply ltsReply = CalypsoRPC.createLTS(bc.getRoster(), bc.getGenesis().getId());
        String secret = "this is a secret";
        WriteRequest wr = new WriteRequest(secret, 16, genesisDarc.getId());
        w = new WriterInstance(bc, Arrays.asList(admin), genesisDarc.getId(), ltsReply, wr);

        Proof p = bc.getProof(w.getInstance().getId());
        assertTrue(p.matches());
    }

    @Test
    void testCopyWriter() throws Exception {
        WriterInstance w2 = new WriterInstance(bc, w.getInstance().getId(), w.getLtsData());
        assertTrue(bc.getProof(w2.getInstance().getId()).matches());
    }
}