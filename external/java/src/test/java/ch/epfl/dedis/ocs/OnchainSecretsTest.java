package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.crypto.Encryption;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.proto.DarcProto;
import ch.epfl.dedis.proto.OCSProto;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
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
    static Darc admin2Darc;
    static Darc publisherDarc;
    static Darc publisher2Darc;
    static Document doc;
    static String docData;
    static String extraData;
    static Document docNew;
    static byte[] readID;

    private final static Logger logger = LoggerFactory.getLogger(OnchainSecretsTest.class);

    @BeforeEach
    void initAll() throws Exception {
        admin = new Ed25519Signer();
        writer = new Ed25519Signer();
        publisher = new Ed25519Signer();
        reader = new Ed25519Signer();
        reader2 = new Ed25519Signer();

        adminDarc = new Darc(admin, null, null);
        adminDarc.AddUser(writer);

        publisherDarc = new Darc(publisher, null, null);
        publisherDarc.AddUser(reader);
        logger.info("publisherDarc: " + DatatypeConverter.printHexBinary(publisherDarc.ID()));
        publisher2Darc = publisherDarc.Copy();
        publisher2Darc.AddUser(reader2);
        publisher2Darc.SetEvolution(publisherDarc, null, publisher);

        admin2Darc = adminDarc.Copy();
        admin2Darc.AddUser(publisherDarc);
        admin2Darc.SetEvolution(adminDarc, null, admin);

        docData = "https://dedis.ch/secret_document.osd";
        doc = new Document(docData, 16, publisherDarc);
        extraData = "created on Monday";
        doc.extraData = extraData.getBytes();

        try {
            logger.info("Admin darc: " + DatatypeConverter.printHexBinary(adminDarc.ID()));
            ocs = new OnchainSecrets(Roster.FromToml(LocalRosters.groupToml), adminDarc);
        } catch (Exception e){
            logger.error("Couldn't start skipchain - perhaps you need to run the following commands:");
            logger.error("cd $GOPATH/github.com/dedis/onchain-secrets/conode");
            logger.error("./run_conode.sh local 3 2");
        }
    }

    @Test
    void verify() throws Exception {
        assertTrue(ocs.verify());
        assertNotNull(ocs.ocsID);
    }

    @Test
    void darcID() throws Exception{
        logger.info("Admin darc after: " + DatatypeConverter.printHexBinary(adminDarc.ID()));
        logger.info("Admin-darc prot: " +
        DatatypeConverter.printHexBinary(adminDarc.ToProto().toByteArray()));
    }

    @Test
    void updateDarc() throws Exception{
        try {
            logger.info("Admin darc after: " + DatatypeConverter.printHexBinary(adminDarc.ID()));
            ocs.updateDarc(adminDarc);
            fail("should not allow to store adminDarc again");
        } catch (Exception e){
            logger.info("correctly failed at updating admin");
        }
        ocs.updateDarc(publisherDarc);
        ocs.updateDarc(publisher2Darc);
        publisher2Darc.AddUser(admin);
        publisher2Darc.SetEvolution(publisherDarc, null, publisher);
        try {
            ocs.updateDarc(publisher2Darc);
        } catch (Exception e){
            logger.info("correctly failed at re-writing publisher darc");
        }
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
        ocs.addIdentityToDarc(adminDarc, IdentityFactory.New(publisher), admin);
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
        docNew = ocs.publishDocument(doc, publisher);
        ocs.updateDarc(publisher2Darc);
    }

    @Test
    void readRequest() throws Exception {
        docNew = ocs.publishDocument(doc, publisher);
        try {
            readID = ocs.readRequest(docNew.id, reader2);
            fail("a wrong read-signature should not pass");
        } catch (CothorityCommunicationException e){
            logger.info("correctly failed with wrong signature");
        }
        logger.debug("publisherdarc.ic = " + DatatypeConverter.printHexBinary(publisherDarc.ID()));
        logger.debug("publisherdarc.proto = " + publisherDarc.ToProto().toString());
        readID = ocs.readRequest(docNew.id, reader);
        assertNotNull(readID);
    }

    @Test
    void getDarcPath() throws Exception{
        ocs.updateDarc(admin2Darc);
        ocs.updateDarc(publisherDarc);
        ocs.updateDarc(publisher2Darc);
        SignaturePath path = ocs.getDarcPath(adminDarc.ID(), IdentityFactory.New(reader2), SignaturePath.USER);
        assertNotNull(path);
        for (Darc d: path.GetDarcs()){
            logger.debug("Darc-list is: " + d.toString());
        }
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
        byte[] readID = ocs.readRequest(docNew.id, reader);
        DecryptKey dk = ocs.decryptKey(readID);
        OCSProto.Write write = ocs.getWrite(docNew.id);
        byte[] keyMaterial = dk.getKeyMaterial(write, reader);
        byte[] data = Encryption.decryptData(write.getData(), keyMaterial);
        logger.info(docData);
        logger.info(DatatypeConverter.printHexBinary(data));
        assertArrayEquals(docData.getBytes(), data);
    }
}