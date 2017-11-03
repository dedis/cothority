package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.crypto.Encryption;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.proto.OCSProto;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.xml.bind.DatatypeConverter;

import static org.junit.jupiter.api.Assertions.*;

class OnchainSecretsTest {
    static OnchainSecrets ocs;
    static Signer admin;
    static Signer writer;
    static Signer publisher;
    static Signer reader;
    static Signer reader2;
    static Darc adminDarc;
    static Darc publisherDarc;
    static Darc publisher2Darc;
    static Document doc;
    static String docData;
    static String extraData;
    static Document docNew;
    static byte[] readID;

    private final static Logger logger = LoggerFactory.getLogger(OnchainSecretsTest.class);

    @BeforeAll
    static void initAll() throws Exception {
        admin = new Ed25519Signer();
        writer = new Ed25519Signer();
        publisher = new Ed25519Signer();
        reader = new Ed25519Signer();
        reader2 = new Ed25519Signer();
        adminDarc = new Darc(admin, null, null);
        publisherDarc = new Darc(publisher, null, null);
        publisherDarc.AddUser(reader);
        logger.info("publisherDarc: " + DatatypeConverter.printHexBinary(publisherDarc.ID()));
        publisher2Darc = publisherDarc.Copy();
        publisher2Darc.AddUser(reader2);
        publisher2Darc.SetEvolution(publisherDarc, null, publisher);
        adminDarc.AddUser(writer);
        docData = "https://dedis.ch/secret_document.osd";
        doc = new Document(docData, 16, publisherDarc);
        extraData = "created on Monday";
        doc.extraData = extraData.getBytes();

        ocs = new OnchainSecrets(Roster.FromToml(LocalRosters.groupToml), adminDarc);
    }

    @Test
    void verify() throws Exception {
        assertTrue(ocs.verify());
        assertNotNull(ocs.ocsID);
    }

    @Test
    void addAccountToSkipchain() throws Exception {
        Signer admin2 = new Ed25519Signer();
        Darc adminDarc2 = adminDarc.Copy();
        adminDarc2.AddOwner(admin2);
        adminDarc2.IncVersion();
        try {
            ocs.updateDarc(adminDarc2);
            fail("Should not update darc without signature");
        } catch (CothorityCommunicationException e){
            logger.info("Correctly refused unsigned darc");
        }
        adminDarc2.SetEvolution(adminDarc, null, admin2);
        try {
            ocs.updateDarc(adminDarc2);
            fail("Should refuse wrong signature");
        } catch (CothorityCommunicationException e){
            logger.info("Correctly refused wrong signature");
        }
        adminDarc2.SetEvolution(adminDarc, null, admin);
        try{
            ocs.updateDarc(adminDarc2);
            logger.info("Accepted correct signature");
        } catch (CothorityCommunicationException e){
            fail("Should accept correct signature");
        }
        logger.info("Updating admin darc again");
        ocs.addIdentityToDarc(adminDarc2, IdentityFactory.New(publisher), admin2);
    }

    @Test
    void getSharedPublicKey() throws Exception {
        Point shared = ocs.getSharedPublicKey();
        assertNotNull(shared);
        assertTrue(ocs.X.equals(shared));
    }

    @Test
    void publishDocument() throws Exception {
        addAccountToSkipchain();
        docNew = ocs.publishDocument(doc, publisher);
        assertNotNull(docNew.id);
    }

    @Test
    void getWrite() throws Exception {
        publishDocument();
        OCSProto.Write write = ocs.getWrite(docNew.id);
        assertEquals(docNew.getWrite(ocs.X).getData(), write.getData());
        assertArrayEquals(extraData.getBytes(), write.getExtradata().toByteArray());
    }

    @Test
    void giveReadAccessToDocument() throws Exception {
        addAccountToSkipchain();
        docNew = ocs.publishDocument(doc, publisher);
        ocs.updateDarc(publisher2Darc);
    }

    @Test
    void readRequest() throws Exception {
        giveReadAccessToDocument();
        logger.info("pd: " + DatatypeConverter.printHexBinary(publisherDarc.ID()));
        readID = ocs.readRequest(docNew.id, publisherDarc, reader);
        assertNotNull(readID);
    }

    @Test
    void readDocument() throws Exception {
        readRequest();
        DecryptKey dk = ocs.decryptKey(readID);
        assertNotNull(dk);
        OCSProto.Write write = ocs.getWrite(docNew.id);
        byte[] keyMaterial = dk.getKeyMaterial(write, reader);
        byte[] data = Encryption.decryptData(write.getData(), keyMaterial);
        assertArrayEquals(docData.getBytes(), data);
    }

    @Test
    void resellerTest() throws Exception{
        // Publish document under publisher
        Document doc = new Document(docData, 16, publisherDarc);
        docNew = ocs.publishDocument(doc, publisher);

        // Create reseller and store his darc on the skipchain
        Signer resellerS = new Ed25519Signer();
        Darc resellerD = new Darc(resellerS, null, null);
        ocs.updateDarc(resellerD);

        // Add the reseller to the document's darc and update it on the skipchain
        // This is the manual way to do it
        Darc publisherDarcNew = publisherDarc.Copy();
        publisherDarcNew.AddUser(new DarcIdentity(resellerD));
        publisherDarcNew.SetEvolution(publisherDarc, null, publisher);
        ocs.updateDarc(publisherDarcNew);

        // Finally add the reader to the reseller's darc
        // This is the more automatic way to do it
        Signer reader = new Ed25519Signer();
        Darc resellerDarc2 = ocs.addIdentityToDarc(resellerD, reader, resellerS);

        // Get the document and decrypt it
        byte[] readID = ocs.readRequest(docNew.id, resellerDarc2, reader);
        DecryptKey dk = ocs.decryptKey(readID);
        OCSProto.Write write = ocs.getWrite(docNew.id);
        byte[] keyMaterial = dk.getKeyMaterial(write, reader);
        byte[] data = Encryption.decryptData(write.getData(), keyMaterial);
        logger.info(docData);
        logger.info(DatatypeConverter.printHexBinary(data));
        assertArrayEquals(docData.getBytes(), data);
    }
}