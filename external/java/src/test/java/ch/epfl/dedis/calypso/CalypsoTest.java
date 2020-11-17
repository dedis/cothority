package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.SignerCounters;
import ch.epfl.dedis.byzcoin.contracts.SecureDarcInstance;
import ch.epfl.dedis.integration.DockerTestServerController;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.crypto.Ed25519Pair;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Rules;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.network.ServerIdentity;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;

import static ch.epfl.dedis.byzcoin.ByzCoinRPCTest.BLOCK_INTERVAL;
import static org.junit.jupiter.api.Assertions.*;

class CalypsoTest {
    static CalypsoRPC calypso;

    private static Signer admin;
    private static Darc genesisDarc;
    private static Signer publisher;
    private static Darc publisherDarc;
    private static Signer reader;
    private static Darc readerDarc;
    private static Ed25519Pair ephemeralPair;

    private static Document doc;
    private static String docData;
    private static String extraData;

    private final static Logger logger = LoggerFactory.getLogger(CalypsoTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        admin = new SignerEd25519();
        publisher = new SignerEd25519();
        reader = new SignerEd25519();
        testInstanceController = TestServerInit.getInstance();
        genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());
        genesisDarc.addIdentity("spawn:" + LTSInstance.ContractId, admin.getIdentity(), Rules.OR);
        genesisDarc.addIdentity("invoke:" + LTSInstance.ContractId + "." + LTSInstance.InvokeCommand, admin.getIdentity(), Rules.OR);

        try {
            logger.info("Admin darc: " + genesisDarc.getBaseId().toString());
            ByzCoinRPC bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, BLOCK_INTERVAL);
            for (ServerIdentity si : bc.getRoster().getNodes()) {
                CalypsoRPC.authorize(si, bc.getGenesisBlock().getId());
            }
            calypso = new CalypsoRPC(bc, genesisDarc.getId(), bc.getRoster(), Collections.singletonList(admin), Collections.singletonList(1L));
            if (!calypso.checkLiveness()) {
                throw new CothorityCommunicationException("liveness check failed");
            }
        } catch (CothorityCommunicationException e) {
            logger.info("Error is: " + e.toString());
            logger.error("Couldn't start skipchain - perhaps you need to run the following commands:");
            logger.error("cd $(go env GOPATH)/src/github.com/dedis/onchain-secrets/conode");
            logger.error("./run_conode.sh local 4 2");
            fail("Couldn't start ocs!");
        }

        readerDarc = new Darc(Arrays.asList(publisher.getIdentity()), Arrays.asList(reader.getIdentity()), "readerDarc".getBytes());
        calypso.getGenesisDarcInstance().spawnDarcAndWait(readerDarc, admin, 2L, 10);

        // Spawn a new darc with the calypso read/write rules for a new signer.
        publisherDarc = new Darc(Arrays.asList(publisher.getIdentity()), Arrays.asList(publisher.getIdentity()), "calypso darc".getBytes());
        publisherDarc.setRule("spawn:calypsoWrite", publisher.getIdentity().toString().getBytes());
        publisherDarc.addIdentity("spawn:calypsoRead", publisher.getIdentity(), Rules.OR);
        publisherDarc.addIdentity("spawn:calypsoRead", readerDarc.getIdentity(), Rules.OR);
        calypso.getGenesisDarcInstance().spawnDarcAndWait(publisherDarc, admin, 3L, 10);

        docData = "https://dedis.ch/secret_document.osd";
        extraData = "created on Monday";
        doc = new Document(docData.getBytes(), extraData.getBytes(), publisherDarc.getBaseId());

        ephemeralPair = new Ed25519Pair();
    }

    @AfterEach
    void restartNodes() {
        try {
            for (int i = 1; i <= 4; i++) {
                testInstanceController.killConode(i);
            }
            testInstanceController.cleanDBs();
            Thread.sleep(1000);
            for (int i = 1; i <= 4; i++) {
                testInstanceController.startConode(i);
            }
        } catch (Exception ignored) {
        }
    }

    // This test creates a full cycle with regard to storing and retrieving a document from Calypso.
    @Test
    void fullCycleDocument() throws CothorityException {
        // The document is stored in 'doc' and not encrypted yet.
        Document doc = new Document(docData.getBytes(), extraData.getBytes(), publisherDarc.getBaseId());

        // First, create an encrypted version of the document. Alternatively one could create
        // an own WriteData from scratch and hand it an already encrypted document.
        // wd holds the encrypted data and the encrypted symmetric key.
        WriteData wd = doc.getWriteData(calypso.getLTS());

        // Now ask Calypso to store it in Byzcoin by creating a WriteInstance.
        WriteInstance wi = new WriteInstance(calypso, publisherDarc.getBaseId(),
                Arrays.asList(publisher), Collections.singletonList(1L),
                wd);

        // The document is now stored on ByzCoin with the data encrypted by the symmetric key (keyMaterial) and the
        // symmetric key encrypted by the Long Term Secret.

        // To read it, first proof that we have the right to read by creating a ReadInstance:
        ReadInstance ri = new ReadInstance(calypso, wi,
                Arrays.asList(reader), Collections.singletonList(1L), ephemeralPair.point);
        // If successful (no exceptions), Byzcoin holds a proof that we are allowed to read the document.

        // Get the re-encrypted symmetric key from Calypso:
        DecryptKeyReply dkr = calypso.tryDecrypt(calypso.getProof(wi.getInstance().getId()), calypso.getProof(ri.getInstance().getId()));
        // And derive the symmetric key, using the ephemeral scalar (the private part) to decrypt it:
        byte[] keyMaterial = dkr.extractKeyMaterial(ephemeralPair.scalar);

        // Finally get the document back:
        Document doc2 = Document.fromWriteInstance(wi, keyMaterial);

        // And check it's the same.
        assertEquals(doc, doc2);

        // If we provide the wrong scalar, then the document should be different.
        // The following might (or might not) throw during key extraction due to invalid point.
        // If it doesn't throw, then we make it throw ourselves when the two documents are equal.
        Ed25519Pair badEphemeralPair = new Ed25519Pair();
        assertThrows(CothorityCryptoException.class, () -> {
            byte[] badKeyMaterial = dkr.extractKeyMaterial(badEphemeralPair.scalar);
            Document badDoc2 = Document.fromWriteInstance(wi, badKeyMaterial);
            if (doc.equals(badDoc2)) {
                throw new CothorityCryptoException("documents should not be equal");
            }
        });
    }

    @Test
    void fullCycleDocumentShort() throws CothorityException {
        // Same as above, but shortest possible calls.
        // Create WriteInstance.
        WriteInstance wi = new WriteInstance(calypso, publisherDarc.getBaseId(),
                Arrays.asList(publisher), Collections.singletonList(1L),
                doc.getWriteData(calypso.getLTS()));

        // Get ReadInstance with 'reader'
        ReadInstance ri = new ReadInstance(calypso, wi, Arrays.asList(reader), Collections.singletonList(1L), ephemeralPair.point);

        // Create new Document from wi and ri
        Document doc2 = Document.fromCalypso(calypso, ri.getInstance().getId(), ephemeralPair.scalar);

        // Should be the same
        assertTrue(doc.equals(doc2));
    }

    @Test
    void decryptKey() throws Exception {
        Document doc1 = new Document("this is secret 1".getBytes(), null, publisherDarc.getBaseId());
        WriteInstance w1 = new WriteInstance(calypso, publisherDarc.getBaseId(),
                Arrays.asList(publisher), Collections.singletonList(1L),
                doc1.getWriteData(calypso.getLTS()));
        ReadInstance r1 = new ReadInstance(calypso, WriteInstance.fromCalypso(calypso, w1.getInstance().getId()),
                Arrays.asList(publisher), Collections.singletonList(2L), ephemeralPair.point);
        Proof pw1 = calypso.getProof(w1.getInstance().getId());
        Proof pr1 = calypso.getProof(r1.getInstance().getId());

        Document doc2 = new Document("this is secret 2".getBytes(), null, publisherDarc.getBaseId());
        WriteInstance w2 = new WriteInstance(calypso, publisherDarc.getBaseId(),
                Arrays.asList(publisher), Collections.singletonList(3L),
                doc2.getWriteData(calypso.getLTS()));
        ReadInstance r2 = new ReadInstance(calypso, WriteInstance.fromCalypso(calypso, w2.getInstance().getId()),
                Arrays.asList(publisher), Collections.singletonList(4L), ephemeralPair.point);
        Proof pw2 = calypso.getProof(w2.getInstance().getId());
        Proof pr2 = calypso.getProof(r2.getInstance().getId());

        try {
            calypso.tryDecrypt(pw2, pr1);
        } catch (CothorityCommunicationException e) {
            assertTrue(e.getMessage().contains("read doesn't point to passed write"));
        }

        try {
            calypso.tryDecrypt(pw1, pr2);
        } catch (CothorityCommunicationException e) {
            assertTrue(e.getMessage().contains("read doesn't point to passed write"));
        }

        logger.info("trying decrypt 1, pk: " + publisher.getPublic().toString());
        DecryptKeyReply dkr1 = calypso.tryDecrypt(pw1, pr1);
        byte[] km1 = dkr1.extractKeyMaterial(ephemeralPair.scalar);
        assertTrue(Arrays.equals(doc1.getData(), Encryption.decryptData(w1.getWrite().getDataEnc(), km1)));

        logger.info("trying decrypt 2, pk: " + publisher.getPublic().toString());
        DecryptKeyReply dkr2 = calypso.tryDecrypt(pw2, pr2);
        byte[] km2 = dkr2.extractKeyMaterial(ephemeralPair.scalar);
        assertTrue(Arrays.equals(doc2.getData(), Encryption.decryptData(w2.getWrite().getDataEnc(), km2)));
    }

    @Test
    void getSharedPublicKey() throws Exception {
        assertThrows(CothorityCommunicationException.class, () -> calypso.getLTSReply(new LTSId(new byte[32])));
        CreateLTSReply lts2 = calypso.getLTSReply(calypso.getLTS().getLTSID());
        assertNotNull(lts2.getX());
        assertTrue(calypso.getLTSX().equals(lts2.getX()));
    }

    @Test
    void getWrite() throws Exception {
        WriteInstance writeInstance = doc.spawnWrite(calypso, publisherDarc.getBaseId(), publisher, 1L);
        WriteInstance writeInstance2 = WriteInstance.fromCalypso(calypso, writeInstance.getInstance().getId());
        assertArrayEquals(doc.getWriteData(calypso.getLTS()).getDataEnc(), writeInstance2.getWrite().getDataEnc());
        assertArrayEquals(doc.getExtraData(), writeInstance2.getWrite().getExtraData());
    }

    ReadInstance readInstance;

    @Test
    void readRequest() throws Exception {
        WriteInstance writeInstance = doc.spawnWrite(calypso, publisherDarc.getBaseId(), publisher, 1L);
        Signer reader2 = new SignerEd25519();
        try {
            readInstance = writeInstance.spawnCalypsoRead(calypso, Arrays.asList(reader2), Collections.singletonList(1L), ephemeralPair.point);
            fail("a wrong read-signature should not pass");
        } catch (CothorityCommunicationException e) {
            logger.info("correctly failed with wrong signature");
        }
        logger.debug("publisherdarc.ic = " + readerDarc.getBaseId().toString());
        logger.debug("publisherdarc.proto = " + readerDarc.toProto().toString());
        readInstance = writeInstance.spawnCalypsoRead(calypso, Arrays.asList(reader), Collections.singletonList(1L), ephemeralPair.point);
        assertNotNull(readInstance);
    }

    @Test
    void readDocument() throws Exception {
        readRequest();
        byte[] keyMaterial = readInstance.decryptKeyMaterial(ephemeralPair.scalar);
        assertNotNull(keyMaterial);
        byte[] data = Encryption.decryptData(doc.getWriteData(calypso.getLTS()).getDataEnc(), keyMaterial);
        assertArrayEquals(docData.getBytes(), data);
    }

    @Test
    void checkFailingWriteAuthorization() throws CothorityException {
        Signer publisher2 = new SignerEd25519();
        try {
            doc.spawnWrite(calypso, publisherDarc.getBaseId(), publisher2, 1L);
            fail("accepted unknown writer");
        } catch (CothorityCommunicationException e) {
            logger.info("correctly refused unknown writer");
        }
    }

    @Test
    void createDarcForTheSameUserInDifferentSkipchain() throws Exception {
        // Get the counter for the admin
        SignerCounters adminCtrs = calypso.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        Darc userDarc = new Darc(Arrays.asList(new SignerEd25519(Hex.parseHexBinary("AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D")).getIdentity()), null, null);
        calypso.getGenesisDarcInstance().spawnDarcAndWait(userDarc, admin, adminCtrs.head() + 1, 10);

        ByzCoinRPC bc2 = new ByzCoinRPC(calypso.getRoster(), genesisDarc, BLOCK_INTERVAL);
        for (ServerIdentity si : bc2.getRoster().getNodes()) {
            CalypsoRPC.authorize(si, bc2.getGenesisBlock().getId());
        }
        CalypsoRPC calypso2 = new CalypsoRPC(bc2, genesisDarc.getId(), bc2.getRoster(), Collections.singletonList(admin), Collections.singletonList(1L));
        try {
            calypso2.getGenesisDarcInstance().spawnDarcAndWait(userDarc, admin, 2L, 10);
            logger.info("correctly saved same darc in another ByzCoin");
        } catch (CothorityCommunicationException e) {
            fail("incorrectly refused to save again");
        }
    }

    @Test
    void writeRequestWithFailedNode() throws Exception {
        WriteData wr = doc.getWriteData(calypso.getLTS());

        // kill the conode co4 and try to make a request
        testInstanceController.killConode(4);
        assertEquals(3, testInstanceController.countRunningConodes());

        try {
            new WriteInstance(calypso, publisherDarc.getBaseId(),
                    Collections.singletonList(publisher), Collections.singletonList(1L),
                    wr);
            logger.info("correctly created write instance");
        } catch (CothorityException e) {
            fail("should not fail to create write instance with one missing node");
        } finally {
            // bring the conode backup for future tests and make sure we have 4 conodes running
            testInstanceController.startConode(4);
            assertEquals(4, testInstanceController.countRunningConodes());
        }

        // Try to write again with 4 nodes
        new WriteInstance(calypso, publisherDarc.getBaseId(),
                Collections.singletonList(publisher), Collections.singletonList(2L),
                wr);
    }

    @Test
    void giveReadAccessToDocument() throws CothorityException {
        WriteInstance wi = doc.spawnWrite(calypso, publisherDarc.getBaseId(), publisher, 1L);

        Signer reader2 = new SignerEd25519();
        try{
            new ReadInstance(calypso, wi, Arrays.asList(reader2), Collections.singletonList(1L), ephemeralPair.point);
            fail("read-request of unauthorized reader should fail");
        } catch (CothorityException e) {
            logger.info("correct refusal of invalid read-request");
        }

        SecureDarcInstance rd = SecureDarcInstance.fromByzCoin(calypso, readerDarc);
        readerDarc.addIdentity(Darc.RuleSignature, reader2.getIdentity(), Rules.OR);
        rd.evolveDarcAndWait(readerDarc, publisher, 2L, 10);

        ReadInstance ri = new ReadInstance(calypso, wi, Arrays.asList(reader2), Collections.singletonList(1L), ephemeralPair.point);
        byte[] keyMaterial = ri.decryptKeyMaterial(ephemeralPair.scalar);
        assertArrayEquals(doc.getKeyMaterial(), keyMaterial);
    }

    @Test
    void getDocument() throws CothorityException {
        WriteInstance wi = doc.spawnWrite(calypso, publisherDarc.getBaseId(), publisher, 1L);
        ReadInstance ri = wi.spawnCalypsoRead(calypso, Arrays.asList(reader), Collections.singletonList(1L), ephemeralPair.point);
        Document doc2 = Document.fromCalypso(calypso, ri.getInstance().getId(), ephemeralPair.scalar);
        assertTrue(doc.equals(doc2));

        // Add another reader
        Signer reader2 = new SignerEd25519();
        SecureDarcInstance di = SecureDarcInstance.fromByzCoin(calypso, readerDarc);
        readerDarc.addIdentity(Darc.RuleSignature, reader2.getIdentity(), Rules.OR);
        di.evolveDarcAndWait(readerDarc, publisher, 2L, 10);

        ReadInstance ri2 = wi.spawnCalypsoRead(calypso, Arrays.asList(reader2), Collections.singletonList(1L), ephemeralPair.point);
        Document doc3 = Document.fromCalypso(calypso, ri2.getInstance().getId(), ephemeralPair.scalar);
        assertTrue(doc.equals(doc3));
    }

    @Test
    void getDocumentWithFailedNode() throws CothorityException, IOException, InterruptedException {
        WriteInstance wr = doc.spawnWrite(calypso, publisherDarc.getBaseId(), publisher, 1L);

        SecureDarcInstance di = SecureDarcInstance.fromByzCoin(calypso, readerDarc);
        Signer reader2 = new SignerEd25519();
        readerDarc.addIdentity(Darc.RuleSignature, reader2.getIdentity(), Rules.OR);
        di.evolveDarcAndWait(readerDarc, publisher, 2L, 10);
        ReadInstance ri = new ReadInstance(calypso, wr, Arrays.asList(reader2), Collections.singletonList(1L), ephemeralPair.point);
        Document doc2 = Document.fromCalypso(calypso, ri.getInstance().getId(), ephemeralPair.scalar);
        assertTrue(doc.equals(doc2));

        // kill the conode co3 and try to make a request
        testInstanceController.killConode(4);
        assertEquals(3, testInstanceController.countRunningConodes());

        ReadInstance ri2 = new ReadInstance(calypso, wr, Arrays.asList(reader2), Collections.singletonList(2L), ephemeralPair.point);
        Document doc3 = Document.fromCalypso(calypso, ri2.getInstance().getId(), ephemeralPair.scalar);
        assertTrue(doc.equals(doc3));

        // restart the conode and try the same
        testInstanceController.startConode(4);
        assertEquals(4, testInstanceController.countRunningConodes());

        ReadInstance ri3 = new ReadInstance(calypso, wr, Arrays.asList(reader2), Collections.singletonList(3L), ephemeralPair.point);
        Document doc4 = Document.fromCalypso(calypso, ri3.getInstance().getId(), ephemeralPair.scalar);
        assertTrue(doc.equals(doc4));
    }

    @Test
    void multiLTS() throws CothorityException {
        CalypsoRPC calypso2 = new CalypsoRPC(calypso, calypso.getGenesisDarc().getBaseId(), calypso.getRoster(),
                Collections.singletonList(admin), Collections.singletonList(4L));
        assertFalse(calypso2.getLTSId().equals(calypso.getLTS().getLTSID()));
    }

    @Test
    void reConnect() throws CothorityException, InterruptedException, IOException {
        WriteInstance wr = doc.spawnWrite(calypso, publisherDarc.getBaseId(), publisher, 1L);

        // TODO: Here the nodes should be shut down and started again, but for some reason that doesn't work.
        // 2020-05-19 - as Java is not our main target anymore, the restarting code has been removed.

        // Reconnect to the ledger.
        ByzCoinRPC bc = ByzCoinRPC.fromByzCoin(calypso.getRoster(), calypso.getGenesisBlock().getSkipchainId());

        // Dropping connection by re-creating an calypso. The following elements are needed:
        // - LTS-id
        // - WriteData-id
        // - reader-signer
        // - publisher-signer
        CalypsoRPC calypso2 = CalypsoRPC.fromCalypso(bc, calypso.getLTSId());
        Signer reader2 = new SignerEd25519();
        SecureDarcInstance di = SecureDarcInstance.fromByzCoin(calypso2, readerDarc);
        readerDarc.addIdentity(Darc.RuleSignature, reader2.getIdentity(), Rules.OR);
        di.evolveDarcAndWait(readerDarc, publisher, 2L, 10);
        ReadInstance ri = new ReadInstance(calypso2, wr, Arrays.asList(reader2), Collections.singletonList(1L), ephemeralPair.point);
        Document doc2 = Document.fromCalypso(calypso2, ri.getInstance().getId(), ephemeralPair.scalar);
        assertTrue(doc.equals(doc2));
    }

    @Test
    void reshareSame() throws Exception {
        // create a new transaction with the same roster
        SignerCounters adminCtrs = calypso.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));
        adminCtrs.increment();
        LTSInstance ltsInst = LTSInstance.fromByzCoin(calypso, calypso.getLTS().getInstanceId());
        ltsInst.reshareLTS(calypso.getRoster(), Collections.singletonList(admin), adminCtrs.getCounters());
        // start the resharing
        Proof proof = ltsInst.getProofAndVerify();
        logger.info("starting resharing");
        calypso.reshareLTS(proof);
        // try to write something to make sure it works
        decryptKey();
    }

    @Test
    void reshareOneMore() throws Exception {
        try {
            // create a new transaction with one more node in the roster
            testInstanceController.startConode(5);
            ServerIdentity conode5 = testInstanceController.getIdentities().get(4);
            CalypsoRPC.authorize(conode5, calypso.getGenesisBlock().getId());

            List<ServerIdentity> newList = calypso.getRoster().getNodes();
            assertTrue(newList.add(conode5));
            Roster newRoster = new Roster(newList);

            SignerCounters adminCtrs = calypso.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));
            adminCtrs.increment();

            LTSInstance ltsInst = LTSInstance.fromByzCoin(calypso, calypso.getLTS().getInstanceId());
            ltsInst.reshareLTS(newRoster, Collections.singletonList(admin), adminCtrs.getCounters());

            // start the resharing
            Proof proof = ltsInst.getProofAndVerify();
            logger.info("starting resharing");
            calypso.reshareLTS(proof);

            // try to write something to make sure it works
            decryptKey();
        } catch (CothorityException e) {
            throw e;
        } finally {
            testInstanceController.killConode(5);
        }
    }
}
