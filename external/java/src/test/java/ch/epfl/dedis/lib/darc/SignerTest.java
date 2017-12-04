package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import org.junit.jupiter.api.Test;

import java.io.IOException;

import static org.junit.jupiter.api.Assertions.*;

public class SignerTest {
    @Test
    void instantiateSigner() throws IOException, Exception{
        Ed25519Signer signer = new Ed25519Signer();
        Point pub = signer.getPublic();
        Scalar priv = signer.getPrivate();

        assertTrue(priv.scalarMult(null).equals(pub));
        assertTrue(pub.equals(SignerFactory.New(signer.serialize()).getPublic()));
    }

    @Test
    void signAndVerify() throws Exception{
        Ed25519Signer signer = new Ed25519Signer();
        byte[] msg = "document".getBytes();

        byte[] sig = signer.sign(msg);
        assertTrue(IdentityFactory.New(signer).verify(msg, sig));
    }

    @Test
    void serialize() throws Exception{
        Ed25519Signer signer = new Ed25519Signer();
        byte[] buf = signer.serialize();
        Signer signer2 = SignerFactory.New(buf);

        assertTrue(signer.getPrivate().equals(signer2.getPrivate()));
        assertTrue(signer.getPublic().equals(signer2.getPublic()));
    }
}
