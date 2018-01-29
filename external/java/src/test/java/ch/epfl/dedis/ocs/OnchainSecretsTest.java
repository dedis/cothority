

package ch.epfl.dedis.ocs;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.LocalRosters;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.crypto.Encryption;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.proto.OCSProto;
import ch.epfl.dedis.proto.SkipBlockProto;
import com.google.protobuf.InvalidProtocolBufferException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class OnchainSecretsTest {
    static OnchainSecrets ocs;
    static Signer admin;
    static Signer publisher;
    static Signer reader;
    static Darc adminDarc;
    static Darc readerDarc;
    static Document doc;
    static String docData;
    static String extraData;

    private final static Logger logger = LoggerFactory.getLogger(OnchainSecretsRPCTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws CothorityException {
        admin = new Ed25519Signer();
        publisher = new Ed25519Signer();
        reader = new Ed25519Signer();

        adminDarc = new Darc(admin, Arrays.asList(publisher), null);
        readerDarc = new Darc(publisher, Arrays.asList(reader), null);

        docData = "https://dedis.ch/secret_document.osd";
        extraData = "created on Monday";
        doc = new Document(docData.getBytes(), 16, readerDarc, extraData.getBytes());

        testInstanceController = TestServerInit.getInstanceManual();

        try {
            logger.info("Admin darc: " + adminDarc.getId().toString());
            ocs = new OnchainSecrets(LocalRosters.FromToml(LocalRosters.groupToml), adminDarc);
        } catch (Exception e){
            logger.error("Couldn't start skipchain - perhaps you need to run the following commands:");
            logger.error("cd $(go env GOPATH)/src/github.com/dedis/onchain-secrets/conode");
            logger.error("./run_conode.sh local 4 2");
        }
    }

    @Test
    void addAccountToSkipchain() throws CothorityException {
        Darc admin3Darc = ocs.addIdentityToDarc(adminDarc, IdentityFactory.New(publisher), admin, 0);
        assertNotNull(admin3Darc);
    }

    @Test
    void publishDocument() throws CothorityException{
        ocs.publishDocument(doc, publisher);
    }

    @Test
    void giveReadAccessToDocument() throws CothorityException {
        Signer reader2 = new Ed25519Signer();
        WriteRequest wr = ocs.publishDocument(doc, publisher);
        try{
            ocs.getDocument(wr.id, reader2);
            fail("read-request of unauthorized reader should fail");
        } catch (CothorityException e){
            logger.info("correct refusal of invalid read-request");
        }
        ocs.addIdentityToDarc(readerDarc, reader2, publisher, SignaturePath.USER);
        Document doc2 = ocs.getDocument(wr.id, reader2);
        assertTrue(doc.equals(doc2));
        // Inverse is not true, as doc2 now contains a writeId
        assertFalse(doc2.equals(doc));
    }

    @Test
    void getDocument() throws CothorityException, IOException, InterruptedException {
        WriteRequest wr = ocs.publishDocument(doc, publisher);
        Document doc2 = ocs.getDocument(wr.id, reader);
        assertTrue(doc.equals(doc2));
        // Inverse is not true, as doc2 now contains a writeId
        assertFalse(doc2.equals(doc));

        // Add another reader
        Signer reader2 = new Ed25519Signer();
        ocs.addIdentityToDarc(readerDarc, reader2, publisher, SignaturePath.USER);
        Document doc3 = ocs.getDocument(wr.id, reader2);
        assertTrue(doc.equals(doc3));
        assertFalse(doc3.equals(doc));
    }

    @Test
    void ephemeralReadDocument() throws Exception{
        WriteRequest write = ocs.publishDocument(doc, publisher);
        Document doc2 = ocs.getDocumentEphemeral(write.id, reader);
        assertTrue(doc.equals(doc2));
    }

    @Test
    void getDocumentWithFailedNode() throws CothorityException, IOException, InterruptedException {
        Signer reader2 = new Ed25519Signer();
        WriteRequest wr = ocs.publishDocument(doc, publisher);

        ocs.addIdentityToDarc(readerDarc, reader2, publisher, SignaturePath.USER);
        Document doc2 = ocs.getDocument(wr.id, reader2);
        assertTrue(doc.equals(doc2));
        // Inverse is not true, as doc2 now contains a writeId
        assertFalse(doc2.equals(doc));

        // kill the conode co3 and try to make a request
        testInstanceController.killConode(4);
        assertEquals(3, testInstanceController.countRunningConodes());

        Document doc3 = ocs.getDocument(wr.id, reader2);
        assertTrue(doc.equals(doc3));
        assertFalse(doc3.equals(doc));

        // restart the conode and try the same
        testInstanceController.startConode(4);
        assertEquals(4, testInstanceController.countRunningConodes());

        Document doc4 = ocs.getDocument(wr.id, reader2);
        assertTrue(doc.equals(doc4));
        assertFalse(doc4.equals(doc));
    }

    @Test
    void reConnect() throws CothorityException {
        WriteRequest wr = ocs.publishDocument(doc, publisher);

        // Dropping connection by re-creating an OCS. The following elements are needed:
        // - roster
        // - ocs-id
        // - WriteRequest-id
        // - reader-signer
        // - publisher-signer
        OnchainSecrets ocs2 = new OnchainSecrets(ocs.getRoster(), ocs.getID());
        Signer reader = new Ed25519Signer();
        OCSProto.Write wr2 = ocs.getWrite(wr.id);
        ocs2.addIdentityToDarc(new Darc(wr2.getReader()), reader, publisher, SignaturePath.USER);
        Document doc2 = ocs2.getDocument(wr.id, reader);
        assertTrue(doc.equals(doc2));
        assertFalse(doc2.equals(doc));
    }

    @Test
    void readDarcs() throws CothorityException, InvalidProtocolBufferException{
        ocs.addIdentityToDarc(adminDarc, publisher, admin, SignaturePath.USER);
        List<Darc> darcs = new ArrayList<>();
        for (SkipblockId latest = ocs.getID();latest != null;){
            SkipBlockProto.SkipBlock sb = ocs.getSkipblock(latest);
            OCSProto.Transaction transaction = OCSProto.Transaction.parseFrom(sb.getData());
            if (transaction.hasDarc()){
                darcs.add(new Darc(transaction.getDarc()));
            }
            if (sb.getForwardCount() > 0) {
                latest = new SkipblockId(sb.getForward(0).getMsg().toByteArray());
            } else {
                latest = null;
            }
        }
        assertEquals(2, darcs.size());
    }
}
