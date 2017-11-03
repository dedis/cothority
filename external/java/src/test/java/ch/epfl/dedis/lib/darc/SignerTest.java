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
        Point pub = signer.GetPublic();
        Scalar priv = signer.GetPrivate();

        assertTrue(priv.scalarMult(null).equals(pub));
        assertTrue(pub.equals(SignerFactory.New(signer.Serialize()).GetPublic()));
    }

    @Test
    void signAndVerify() throws Exception{
        Ed25519Signer signer = new Ed25519Signer();
        byte[] msg = "document".getBytes();

        byte[] sig = signer.Sign(msg);
        assertTrue(IdentityFactory.New(signer).Verify(msg, sig));
    }

    @Test
    void serialize() throws Exception{
        Ed25519Signer signer = new Ed25519Signer();
        byte[] buf = signer.Serialize();
        Signer signer2 = SignerFactory.New(buf);

        assertTrue(signer.GetPrivate().equals(signer2.GetPrivate()));
        assertTrue(signer.GetPublic().equals(signer2.GetPublic()));
    }
}
