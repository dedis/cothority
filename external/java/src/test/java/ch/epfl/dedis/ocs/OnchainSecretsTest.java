package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.CothorityCommunicationException;
import ch.epfl.dedis.lib.Crypto;
import ch.epfl.dedis.lib.DecryptKey;
import ch.epfl.dedis.proto.OCSProto;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import javax.crypto.Cipher;
import javax.crypto.spec.IvParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.security.SecureRandom;
import java.util.List;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.*;

class OnchainSecretsTest {
    static OnchainSecrets ocs;
    static Account admin;
    static Account publisher;
    static Account reader;
    static Document doc;
    static String docData;
    static Document docNew;
    static byte[] readID;

    @BeforeAll
    static void initAll() throws Exception {
        OcsFactory ocsFactory = new OcsFactory()
                .addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1)
                .addConode(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2)
                .addConode(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3)
                .initialiseNewChain();

        ocs = ocsFactory.createConnection();

        //ocs = ConnectingWithTomlConfig.connectClusterWithTomlConfig(LocalRosters.groupToml);

        admin = new Account(Account.ADMIN);
        publisher = new Account(Account.WRITER);
        reader = new Account(Account.READER);
        docData = "https://dedis.ch/secret_document.osd";
        doc = new Document(docData, 16);
    }

    @Test
    void verify() throws Exception {
        assertTrue(ocs.verify());
        assertNotNull(ocs.ocsID);
    }

    @Test
    void addAccountToSkipchain() throws Exception {
        ocs.addAccountToSkipchain(admin, admin);
        try {
            ocs.addAccountToSkipchain(publisher, publisher);
//            fail("this should not be possible");
            System.out.println("this should not be possible");
        } catch (CothorityCommunicationException e) {
            System.out.println(e);
        }
        ocs.addAccountToSkipchain(admin, publisher);
        ocs.addAccountToSkipchain(admin, reader);
    }

    @Test
    void getSharedPublicKey() throws Exception {
        Crypto.Point shared = ocs.getSharedPublicKey();
        assertNotNull(shared);
        assertTrue(ocs.X.equals(shared));
    }

    @Test
    void testEncryption() throws Exception {
        byte[] orig = "My cool file".getBytes();
        byte[] symmetricKey = new byte[16];
        int ivSize = 16;
        byte[] iv = new byte[ivSize];
        SecureRandom random = new SecureRandom();
        random.nextBytes(iv);
        IvParameterSpec ivParameterSpec = new IvParameterSpec(iv);
        random.nextBytes(symmetricKey);
        Cipher cipher = Cipher.getInstance(Document.algo);
        SecretKeySpec key = new SecretKeySpec(symmetricKey, Document.algoKey);
        cipher.init(Cipher.ENCRYPT_MODE, key, ivParameterSpec);
        byte[] data_enc = cipher.doFinal(orig);

        cipher.init(Cipher.DECRYPT_MODE, key, ivParameterSpec);
        byte[] data = cipher.doFinal(data_enc);
        assertArrayEquals(orig, data);
    }

    @Test
    void testDocumentEncryption()throws Exception{
        byte[] orig = "foo beats bar".getBytes();
        byte[] keyMaterial = new byte[Document.ivLength + 16];
        new SecureRandom().nextBytes(keyMaterial);

        byte[] dataEnc = Document.encryptData(orig, keyMaterial);
        byte[] data = Document.decryptData(dataEnc, keyMaterial);
        assertArrayEquals(orig, data);
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
        OCSProto.OCSWrite write = ocs.getWrite(docNew.id);
        assertEquals(docNew.getWrite(ocs.X), write);
    }

    @Test
    void readDarc() throws Exception {
        ocs.addAccountToSkipchain(admin, admin);
        List<Darc> a = ocs.readDarc(admin.ID, false);
        assertEquals(1, a.size());
        Darc admin_copy = a.get(0);
        assertArrayEquals(admin.ID, admin_copy.id);
        assertArrayEquals(admin.Point.toBytes(), admin_copy.points.get(0).toBytes());
    }

    @Test
    void giveReadAccessToDocument() throws Exception {
        addAccountToSkipchain();
        docNew = ocs.publishDocument(doc, publisher);
        ocs.giveReadAccessToDocument(docNew, publisher, reader);
        List<Darc> darcs = ocs.readDarc(docNew.owner.id, false);
        Darc darc = darcs.get(0);
        assertEquals(1, darc.version);
        assertEquals(1, darc.points.size());
        assertTrue(reader.Point.equals(darc.points.get(0)));
    }

    @Test
    void readRequest() throws Exception {
        giveReadAccessToDocument();
        readID = ocs.readRequest(docNew.id, reader);
        assertNotNull(readID);
    }

    @Test
    void readDocument() throws Exception {
        readRequest();
        DecryptKey dk = ocs.decryptKey(readID);
        assertNotNull(dk);
        OCSProto.OCSWrite write = ocs.getWrite(docNew.id);
        byte[] keyMaterial = dk.getKeyMaterial(write, reader);
        byte[] data = Document.decryptData(write.getData(), keyMaterial);
        assertArrayEquals(docData.getBytes(), data);
    }
}