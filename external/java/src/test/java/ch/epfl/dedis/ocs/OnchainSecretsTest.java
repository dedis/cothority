package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.CothorityError;
import ch.epfl.dedis.lib.Crypto;
import ch.epfl.dedis.lib.DecryptKey;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import javax.crypto.Cipher;
import javax.crypto.spec.SecretKeySpec;
import java.util.List;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.*;

class OnchainSecretsTest {
    static OnchainSecrets ocs;
    static Account admin;
    static Account publisher;
    static Account reader;
    static Document doc;
    static Document docNew;
    static byte[] readID;

    @BeforeAll
    static void initAll() throws Exception {
        ocs = new OnchainSecrets(LocalRosters.group);
        admin = new Account(Account.ADMIN);
        publisher = new Account(Account.WRITER);
        reader = new Account(Account.READER);
        doc = new Document("https://dedis.ch/secret_document.osd", 16);
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
        } catch (CothorityError e) {
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
        new Random().nextBytes(symmetricKey);
        Cipher cipher = Cipher.getInstance(Crypto.algo);
        SecretKeySpec key = new SecretKeySpec(symmetricKey, Crypto.algoKey);
        cipher.init(Cipher.ENCRYPT_MODE, key);
        byte[] data_enc = cipher.doFinal(orig);

        cipher.init(Cipher.DECRYPT_MODE, key);
        byte[] data = cipher.doFinal(data_enc);
        assertArrayEquals(orig, data);
    }

    @Test
    void publishDocument() throws Exception {
        addAccountToSkipchain();
        docNew = ocs.publishDocument(doc, publisher);
        assertNotNull(docNew.id);
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
        List<Darc> darcs = ocs.readDarc(docNew.readers.id, false);
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
        dk.X = ocs.X;
        byte[] data = dk.decryptDocument(docNew.ocswrite, reader);
        assertArrayEquals(doc.data, data);
    }

}