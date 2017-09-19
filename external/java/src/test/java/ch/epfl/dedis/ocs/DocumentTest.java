package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Crypto;
import org.junit.jupiter.api.Test;

class DocumentTest {
    @Test
    void getWrite() throws Exception{
        Crypto.KeyPair kp = new Crypto.KeyPair();
        Document doc = new Document("This is a test message", 16);
    }
}
