import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import javax.xml.bind.DatatypeConverter;

import java.security.PublicKey;

import static org.junit.jupiter.api.Assertions.*;

class OnchainSecretsTest {
    static OnchainSecrets ocs;
    static Account admin;
    static Account writer;
    static Account reader;

    @BeforeAll
    static void initAll() throws Exception{
        ocs = new OnchainSecrets(LocalRosters.group);
        admin = new Account(Account.AccessRight.ADMIN);
        writer = new Account(Account.AccessRight.WRITER);
        reader = new Account(Account.AccessRight.READER);
    }

    @Test
    void verify() throws Exception {
        assertTrue(ocs.verify());
        assertNotNull(ocs.ocs_id);
        System.out.println(DatatypeConverter.printHexBinary(ocs.ocs_id));
    }

    @Test
    void addAccountToSkipchain() throws Exception {
        ocs.addAccountToSkipchain(admin, admin);
        try{
            ocs.addAccountToSkipchain(writer, writer);
//            fail("this should not be possible");
            System.out.println("this should not be possible");
        } catch (OnchainSecrets.CothorityError e){
            System.out.println(e);
        }
        ocs.addAccountToSkipchain(admin, writer);
        ocs.addAccountToSkipchain(admin, reader);
    }

    @Test
    void getSharedPublicKey() throws Exception{
        PublicKey shared = ocs.getSharedPublicKey();
        assertNotNull(shared);
        System.out.println("Public shared key: " + Crypto.toString(shared));
        assertArrayEquals(Crypto.toBytes(ocs.X), Crypto.toBytes(shared));
    }

    @Test
    void publishDocument() throws Exception{
        addAccountToSkipchain();
        Document doc = new Document("https://dedis.ch/secret_document.osd");

    }

    @Test
    void readDarc() {
    }

    @Test
    void giveReadAcccessToDocument() {
    }

    @Test
    void readRequest() {
    }

    @Test
    void getSkipblock() {
    }

    @Test
    void decryptKey() {
    }

    @Test
    void readDocument() {
    }

}