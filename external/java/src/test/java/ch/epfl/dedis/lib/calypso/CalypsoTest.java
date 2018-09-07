package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.crypto.Encryption;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.Argument;
import ch.epfl.dedis.lib.omniledger.InstanceId;
import ch.epfl.dedis.lib.omniledger.OmniledgerRPC;
import ch.epfl.dedis.lib.omniledger.Proof;
import ch.epfl.dedis.lib.omniledger.contracts.DarcInstance;
import ch.epfl.dedis.lib.omniledger.darc.Darc;
import ch.epfl.dedis.lib.omniledger.darc.Rules;
import ch.epfl.dedis.lib.omniledger.darc.Signer;
import ch.epfl.dedis.lib.omniledger.darc.SignerEd25519;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.Arrays;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.*;

class CalypsoTest {
    class Pair<A, B> {
        A a;
        B b;
        Pair(A a, B b) {
            this.a = a;
            this.b = b;
        }
    }
    private OmniledgerRPC ol;
    private CreateLTSReply ltsReply;
    private Darc testDarc;
    private Signer testSigner;

    private final static Logger logger = LoggerFactory.getLogger(WriterInstanceTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        Signer admin = new SignerEd25519();
        Rules rules = Darc.initRules(Arrays.asList(admin.getIdentity()),
                Arrays.asList(admin.getIdentity()));
        rules.addRule("spawn:darc", admin.getIdentity().toString().getBytes());
        Darc genesisDarc = new Darc(rules, "genesis".getBytes());
        ol = new OmniledgerRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(500, MILLIS));
        if (!ol.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }

        // Spawn a new darc with the calypso read/write rules for a new signer.
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        testSigner = new SignerEd25519();
        testDarc = new Darc(Arrays.asList(testSigner.getIdentity()), Arrays.asList(testSigner.getIdentity()), "calypso darc".getBytes());
        testDarc.setRule("spawn:calypsoWrite", testSigner.getIdentity().toString().getBytes());
        testDarc.setRule("spawn:calypsoRead", testSigner.getIdentity().toString().getBytes());
        dc.spawnContractAndWait("darc", admin, Argument.NewList("darc", testDarc.toProto().toByteArray()), 10);

        // Run the DKG.
        ltsReply = CalypsoRPC.createLTS(ol.getRoster(), ol.getGenesis().getId());
    }

    @Test
    void testDecryptKey() throws Exception {
        String secret1 = "this is secret 1";
        Pair<WriteRequest, WriterInstance> w1 = createWriterInstance(secret1);
        Pair<ReadRequest, ReaderInstance> r1 = createReaderInstance(w1.b.getInstance().getId());
        Proof pw1 = ol.getProof(w1.b.getInstance().getId());
        Proof pr1 = ol.getProof(r1.b.getInstance().getId());

        String secret2 = "this is secret 2";
        Pair<WriteRequest, WriterInstance> w2 = createWriterInstance(secret2);
        Pair<ReadRequest, ReaderInstance> r2 = createReaderInstance(w2.b.getInstance().getId());
        Proof pw2 = ol.getProof(w2.b.getInstance().getId());
        Proof pr2 = ol.getProof(r2.b.getInstance().getId());

        try {
            CalypsoRPC.tryDecrypt(pr1, pw2, ol.getRoster());
        } catch (CothorityCommunicationException e) {
            assertTrue(e.getMessage().contains("read doesn't point to passed write"));
        }

        try {
            CalypsoRPC.tryDecrypt(pr2, pw1, ol.getRoster());
        } catch (CothorityCommunicationException e) {
            assertTrue(e.getMessage().contains("read doesn't point to passed write"));
        }

        logger.info("trying decrypt 1, pk: " + testSigner.getPublic().toString());
        byte[] key1 = getKeyMaterial(pr1, pw1, testSigner.getPrivate());
        assertTrue(Arrays.equals(secret1.getBytes(), Encryption.decryptData(w1.a.getDataEnc(), key1)));

        logger.info("trying decrypt 2, pk: " + testSigner.getPublic().toString());
        byte[] key2 = getKeyMaterial(pr2, pw2, testSigner.getPrivate());
        assertTrue(Arrays.equals(secret2.getBytes(), Encryption.decryptData(w2.a.getDataEnc(), key2)));
    }

    Pair<WriteRequest, WriterInstance> createWriterInstance(String secret) throws Exception {
        WriteRequest wr = new WriteRequest(secret, 32, testDarc.getId());
        WriterInstance w = new WriterInstance(ol, Arrays.asList(testSigner), testDarc.getId(), ltsReply, wr);

        Proof p = ol.getProof(w.getInstance().getId());
        assertTrue(p.matches());

        return new Pair(wr, w);
    }

    Pair<ReadRequest, ReaderInstance> createReaderInstance(InstanceId writerId) throws Exception {
        ReadRequest rr = new ReadRequest(writerId, testSigner.getPublic());
        ReaderInstance r = new ReaderInstance(ol, Arrays.asList(testSigner), testDarc.getId(), rr);
        assertTrue(ol.getProof(r.getInstance().getId()).matches());
        return new Pair(rr, r);
    }

    byte[] getKeyMaterial(Proof readProof, Proof writeProof, Scalar secret) throws Exception {
        DecryptKeyReply dkr = CalypsoRPC.tryDecrypt(readProof, writeProof, ol.getRoster());
        return dkr.getKeyMaterial(secret);
    }
}