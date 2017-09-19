import org.junit.jupiter.api.Test;
import proto.OCSProto;

class DocumentTest {
    @Test
    void getWrite() throws Exception{
        Crypto.KeyPair kp = new Crypto.KeyPair();
        Document doc = new Document("This is a test message", 16);
        OCSProto.OCSWrite w = doc.getWrite(kp.Point);
    }

}