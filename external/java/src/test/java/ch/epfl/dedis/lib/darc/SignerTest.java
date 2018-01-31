package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.Test;

import java.io.IOException;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

public class SignerTest {
    @Test
    void instantiateSigner() throws IOException, Exception {
        Ed25519Signer signer = new Ed25519Signer();
        Point pub = signer.getPublic();
        Scalar priv = signer.getPrivate();

        assertTrue(priv.scalarMult(null).equals(pub));
        assertTrue(pub.equals(SignerFactory.New(signer.serialize()).getPublic()));
    }

    @Test
    void signAndVerify() throws Exception {
        Ed25519Signer signer = new Ed25519Signer();
        byte[] msg = "document".getBytes();

        byte[] sig = signer.sign(msg);
        assertTrue(IdentityFactory.New(signer).verify(msg, sig));
    }

    @Test
    void serialize() throws Exception {
        Ed25519Signer signer = new Ed25519Signer();
        byte[] buf = signer.serialize();
        Signer signer2 = SignerFactory.New(buf);

        assertTrue(signer.getPrivate().equals(signer2.getPrivate()));
        assertTrue(signer.getPublic().equals(signer2.getPublic()));
    }

    @Test
    void keycard() throws Exception {
        KeycardSigner signer = new TestKeycardSigner("secp256k1-pkcs8.der", "secp256k1-pub.der");
        KeycardSigner signer2 = new TestKeycardSigner("secp256k1-pkcs8.der", "secp256k1-pub.der");

        assertThrows(CothorityCryptoException.class, () -> signer.getPrivate());
        assertThrows(IllegalStateException.class, () -> signer.serialize());

        byte[] msg = "test data to sign".getBytes();
        byte[] sig = signer.sign(msg);
        assertTrue(signer.getIdentity().verify(msg, sig));
        assertTrue(signer2.getIdentity().verify(msg, sig));

        Identity sig1 = signer.getIdentity();
        Identity sig2 = IdentityFactory.New(sig1.toProto());
        assertTrue(sig1.equals(sig2));
        assertTrue(sig1.verify(msg, sig));
        assertTrue(sig2.verify(msg, sig));
    }
}
