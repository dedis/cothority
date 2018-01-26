package ch.epfl.dedis.ocs;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.LocalRosters;
import ch.epfl.dedis.lib.crypto.Encryption;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.proto.OCSProto;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.xml.bind.DatatypeConverter;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class OnchainSecretsRPCTest {
    static OnchainSecretsRPC ocs;
    static Signer admin;
    static Signer publisher;
    static Signer reader;
    static Signer reader2;
    static Darc adminDarc;
    static Darc readerDarc;
    static Darc reader2Darc;
    static WriteRequest writeRequest;
    static String docData;
    static String extraData;
    static WriteRequest writeRequest2;
    static ReadRequestId readID;

    private final static Logger logger = LoggerFactory.getLogger(OnchainSecretsTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        admin = new Ed25519Signer();
        publisher = new Ed25519Signer();
        reader = new Ed25519Signer();
        reader2 = new Ed25519Signer();

        adminDarc = new Darc(admin, null, null);
        adminDarc.addUser(publisher);

        readerDarc = new Darc(publisher, null, null);
        readerDarc.addUser(reader);
        logger.info("readerDarc: " + readerDarc.getId().toString());
        reader2Darc = readerDarc.copy();
        reader2Darc.addUser(reader2);
        reader2Darc.setEvolution(readerDarc, null, publisher);

        docData = "https://dedis.ch/secret_document.osd";
        writeRequest = new WriteRequest(docData, 16, readerDarc);
        extraData = "created on Monday";
        writeRequest.extraData = extraData.getBytes();

        testInstanceController = TestServerInit.getInstance();

        try {
            TestServerInit.getInstance();
            logger.info("Admin darc: " + adminDarc.getId().toString());
            ocs = new OnchainSecretsRPC(LocalRosters.FromToml(LocalRosters.groupToml), adminDarc);
        } catch (CothorityCommunicationException e) {
            logger.info("Error is: " + e.toString());
            logger.error("Couldn't start skipchain - perhaps you need to run the following commands:");
            logger.error("cd $(go env GOPATH)/src/github.com/dedis/onchain-secrets/conode");
            logger.error("./run_conode.sh local 4 2");
            fail("Couldn't start ocs!");
        }
    }

    @Test
    void verify() {
        assertTrue(ocs.verify());
        assertNotNull(ocs.getID());
    }

    @Test
    void darcID() throws Exception {
        logger.info("Admin darc after: " + adminDarc.getId().toString());
        logger.info("Admin-darc prot: " +
                DatatypeConverter.printHexBinary(adminDarc.toProto().toByteArray()));
    }

    @Test
    void updateDarc() throws Exception {
        try {
            logger.info("Admin darc after: " + adminDarc.getId().toString());
            ocs.updateDarc(adminDarc);
            fail("should not allow to store adminDarc again");
        } catch (Exception e) {
            logger.info("correctly failed at updating admin");
        }
        ocs.updateDarc(readerDarc);
        ocs.updateDarc(reader2Darc);
        reader2Darc.addUser(admin);
        reader2Darc.setEvolution(readerDarc, null, publisher);
        try {
            ocs.updateDarc(reader2Darc);
        } catch (Exception e) {
            logger.info("correctly failed at re-writing publisher darc");
        }
    }

    @Test
    void addAccountToSkipchain() throws Exception {
        Signer admin2 = new Ed25519Signer();
        Darc adminDarc2 = adminDarc.copy();
        adminDarc2.addOwner(admin2);
        adminDarc2.incVersion();
        try {
            ocs.updateDarc(adminDarc2);
            fail("Should not update darc without signature");
        } catch (CothorityCommunicationException e) {
            logger.info("Correctly refused unsigned darc");
        }
        adminDarc2.setEvolution(adminDarc, null, admin);
        try {
            ocs.updateDarc(adminDarc2);
            logger.info("Accepted correct signature");
        } catch (CothorityCommunicationException e) {
            fail("Should accept correct signature");
        }
        logger.info("Updating admin darc again");
    }

    @Test
    void getSharedPublicKey() throws Exception {
        Point shared = ocs.getSharedPublicKey();
        assertNotNull(shared);
        assertTrue(ocs.getX().equals(shared));
    }

    @Test
    void publishDocument() throws Exception {
        DarcSignature sig = writeRequest.getSignature(ocs, publisher);
        writeRequest2 = ocs.createWriteRequest(writeRequest, sig);
        assertNotNull(writeRequest2.id);
    }

    @Test
    void getWrite() throws Exception {
        publishDocument();
        OCSProto.Write write = ocs.getWrite(writeRequest2.id);
        assertEquals(writeRequest2.toProto(ocs.getX(), ocs.ocsID).getData(), write.getData());
        assertArrayEquals(extraData.getBytes(), write.getExtradata().toByteArray());
    }

    @Test
    void giveReadAccessToDocument() throws Exception {
        DarcSignature sig = writeRequest.getSignature(ocs, publisher);
        writeRequest2 = ocs.createWriteRequest(writeRequest, sig);
        ocs.updateDarc(reader2Darc);
    }

    @Test
    void readRequest() throws Exception {
        DarcSignature sig = writeRequest.getSignature(ocs, publisher);
        writeRequest2 = ocs.createWriteRequest(writeRequest, sig);
        try {
            readID = ocs.createReadRequest(new ReadRequest(ocs, writeRequest2.id, reader2));
            fail("a wrong read-signature should not pass");
        } catch (CothorityCommunicationException e) {
            logger.info("correctly failed with wrong signature");
        }
        logger.debug("publisherdarc.ic = " + readerDarc.getId().toString());
        logger.debug("publisherdarc.proto = " + readerDarc.toProto().toString());
        readID = ocs.createReadRequest(new ReadRequest(ocs, writeRequest2.id, reader));
        assertNotNull(readID);
    }

    @Test
    void getDarcPath() throws Exception {
        ocs.updateDarc(readerDarc);
        ocs.updateDarc(reader2Darc);
        Darc admin2Darc = adminDarc.copy();
        admin2Darc.addUser(readerDarc);
        admin2Darc.setEvolution(adminDarc, null, admin);
        ocs.updateDarc(admin2Darc);
        SignaturePath path = ocs.getDarcPath(adminDarc.getId(), reader2.getIdentity(), SignaturePath.USER);
        assertNotNull(path);
        for (Darc d : path.getDarcs()) {
            logger.debug("Darc-list is: " + d.toString());
        }
    }

    @Test
    void readDocument() throws Exception {
        readRequest();
        DecryptKey dk = ocs.getDecryptionKey(readID);
        assertNotNull(dk);
        OCSProto.Write write = ocs.getWrite(writeRequest2.id);
        byte[] keyMaterial = dk.getKeyMaterial(write, reader.getPrivate());
        byte[] data = Encryption.decryptData(write.getData(), keyMaterial);
        assertArrayEquals(docData.getBytes(), data);
    }

    @Test
    void getLatestDarc() throws CothorityException {
        Signer publisher2 = new Ed25519Signer();
        Darc admin2Darc = adminDarc.copy();
        admin2Darc.addUser(publisher2);
        admin2Darc.setEvolution(adminDarc, null, admin);
        assertTrue(adminDarc.getBaseId().equals(admin2Darc.getBaseId()));
        logger.info(adminDarc.getBaseId().toString());
        logger.info(admin2Darc.getBaseId().toString());
        ocs.updateDarc(admin2Darc);
        List<Darc> darcs = ocs.getLatestDarc(adminDarc.getId());
        assertEquals(2, darcs.size());
        assertTrue(adminDarc.equals(darcs.get(0)));
        assertTrue(admin2Darc.equals(darcs.get(1)));
    }

    @Test
    void checkWriteAuthorization() throws CothorityException {
        Signer publisher2 = new Ed25519Signer();
        DarcSignature sig = new DarcSignature(writeRequest.owner.getId().getId(),
                writeRequest.owner, publisher2, SignaturePath.USER);
        try {
            writeRequest2 = ocs.createWriteRequest(writeRequest, sig);
            fail("accepted unknown writer");
        } catch (CothorityCommunicationException e) {
            logger.info("correctly refused unknown writer");
        }
    }

    @Test
    void createDarcForTheSameUserInDifferentSkipchain() throws Exception {
        Darc userDarc = new Darc(new Ed25519Signer(DatatypeConverter.parseHexBinary("AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D")), null, null);
        ocs.updateDarc(userDarc);

        OnchainSecretsRPC ocs2 = new OnchainSecretsRPC(LocalRosters.FromToml(LocalRosters.groupToml), adminDarc);
        try {
            ocs2.updateDarc(userDarc);
            fail("should not be able to store darc again");
        } catch (CothorityCommunicationException e) {
            logger.info("correctly refusing to save again");
        }

        Darc userDarc2 = new Darc(new Ed25519Signer(DatatypeConverter.parseHexBinary("AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D")), null, null);
        ocs2.updateDarc(userDarc2);
        logger.info("new user darc created and stored");
    }

    @Test
    void writeRequestWithFailedNode() throws Exception {
        WriteRequest wr = new WriteRequest("data data", 16, readerDarc);
        wr.extraData = "created on Monday".getBytes();
        assertNull(wr.id);

        DarcSignature sig = wr.getSignature(ocs, publisher);
        wr = ocs.createWriteRequest(wr, sig);
        assertNotNull(wr.id);

        // kill the conode co4 and try to make a request
        testInstanceController.killConode(4);
        assertEquals(3, testInstanceController.countRunningConodes());

        wr.id = null;
        wr = ocs.createWriteRequest(wr, sig);
        assertNotNull(wr.id);

        // bring the conode backup for future tests and make sure we have 4 conodes running
        testInstanceController.startConode(4);
        assertEquals(4, testInstanceController.countRunningConodes());

        // try to write again
        wr.id = null;
        wr = ocs.createWriteRequest(wr, sig);
        assertNotNull(wr.id);
    }
}
