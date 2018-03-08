package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.crypto.KeyPair;
import ch.epfl.dedis.lib.darc.Darc;
import org.junit.jupiter.api.Test;

class DocumentTest {
    @Test
    void getWrite() throws Exception{
        KeyPair kp = new KeyPair();
        WriteRequest doc = new WriteRequest("This is a test message", 16, new Darc());
    }
}
