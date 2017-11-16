package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertTrue;

public class SignaturePathTest {
    @Test
    void serialize() throws Exception{
        Ed25519Signer signer = new Ed25519Signer();
        Darc darc = new Darc(signer, null, null);
        SignaturePath path = new SignaturePath(darc, signer, SignaturePath.OWNER);

        DarcProto.SignaturePath proto = path.toProto();
        SignaturePath path2 = new SignaturePath(proto);

        assertTrue(path.equals(path2));
    }
}
