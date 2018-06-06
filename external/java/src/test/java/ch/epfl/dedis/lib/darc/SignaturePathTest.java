package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcOCSProto;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertTrue;

public class SignaturePathTest {
    @Test
    void serialize() throws Exception{
        SignerEd25519 signer = new SignerEd25519();
        Darc darc = new Darc(signer, null, null);
        SignaturePath path = new SignaturePath(darc, signer, SignaturePath.OWNER);

        DarcOCSProto.SignaturePath proto = path.toProto();
        SignaturePath path2 = new SignaturePath(proto);

        assertTrue(path.equals(path2));
    }
}
