import com.google.protobuf.ByteString;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import proto.OCSProto;

import javax.crypto.BadPaddingException;
import javax.crypto.Cipher;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import javax.crypto.spec.SecretKeySpec;
import javax.xml.bind.DatatypeConverter;

import java.io.IOException;
import java.io.UnsupportedEncodingException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.List;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.*;

class OnchainSecretsTest {
    static OnchainSecrets ocs;
    static Account admin;
    static Account publisher;
    static Account reader;
    static Document doc;
    static Document doc_new;
    static byte[] read_id;

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
        assertNotNull(ocs.ocs_id);
    }

    @Test
    void addAccountToSkipchain() throws Exception {
        ocs.addAccountToSkipchain(admin, admin);
        try {
            ocs.addAccountToSkipchain(publisher, publisher);
//            fail("this should not be possible");
            System.out.println("this should not be possible");
        } catch (OnchainSecrets.CothorityError e) {
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
        doc_new = ocs.publishDocument(doc, publisher);
        assertNotNull(doc_new.id);
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
        doc_new = ocs.publishDocument(doc, publisher);
        ocs.giveReadAccessToDocument(doc_new, publisher, reader);
        List<Darc> darcs = ocs.readDarc(doc_new.readers.id, false);
        Darc darc = darcs.get(0);
        assertEquals(1, darc.version);
        assertEquals(1, darc.points.size());
        assertTrue(reader.Point.equals(darc.points.get(0)));
    }

    @Test
    void readRequest() throws Exception {
        giveReadAccessToDocument();
        read_id = ocs.readRequest(doc_new, reader);
        assertNotNull(read_id);
    }

    @Test
    void readDocument() throws Exception {
        readRequest();
        DecryptKey dk = ocs.decryptKey(read_id);
        assertNotNull(dk);
        dk.X = ocs.X;
        byte[] data = dk.decryptDocument(doc_new.ocswrite, reader);
        assertArrayEquals(doc.data, data);
    }

//    @Test
    void precalculated() throws Exception{
//        xc: 762755eb09f5a1b3927d89625a90ac93351eba404aa0d0a62315985cc94ba304
//        xcInv: 77aca071106e70a4431f6e4084693281cae145bfb55f2f59dcea67a336b45c0b
//        X: c76cceeefe5446902f765af4c81aaeff15a5e96eb3e4bbf300c29e6ef631e9ba
//        XhatDec: 36e58644a6696592027a5ccaf2a0ca22d5770bb3506e683078949345d23eca4c
//        XhatEnc: 90dfb2dee183ea69f3d68f3cd4e7a3b7e89ec9c9cbdc99391263dcdcae8126ee
//        Xhat: 3695be3735c3938478717836d625d90637f5370bcf449be8e5ce5eaaddc97d52
//        XhatInv: 3695be3735c3938478717836d625d90637f5370bcf449be8e5ce5eaaddc97dd2
//        C: 4a89a0a4440818f66f4a38ea0285e8e38cd0cf7a148b60c4c7fa7610ad2abfe9
//        keyPointHat: 10294aeda9694e0391eec2d8c133bebbff000000000000000000000000000007
//        keyPart: [41 74 237 169 105 78 3 145 238 194 216 193 51 190 187 255]

        OCSProto.OCSWrite.Builder write = OCSProto.OCSWrite.newBuilder();
        write.setData(ByteString.copyFrom("test".getBytes()));
        write.addCs(new Crypto.Point("4a89a0a4440818f66f4a38ea0285e8e38cd0cf7a148b60c4c7fa7610ad2abfe9").toProto());
        write.setU(new Crypto.Point("4a89a0a4440818f66f4a38ea0285e8e38cd0cf7a148b60c4c7fa7610ad2abfe9").toProto());

        DecryptKey dk = new DecryptKey();
        dk.Cs.add(new Crypto.Point("4a89a0a4440818f66f4a38ea0285e8e38cd0cf7a148b60c4c7fa7610ad2abfe9"));
        dk.XhatEnc = new Crypto.Point("90dfb2dee183ea69f3d68f3cd4e7a3b7e89ec9c9cbdc99391263dcdcae8126ee");
        dk.X = new Crypto.Point("c76cceeefe5446902f765af4c81aaeff15a5e96eb3e4bbf300c29e6ef631e9ba");
        byte[] data = dk.decryptDocument(write.build(), reader);
    }
}