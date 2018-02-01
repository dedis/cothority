package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.Test;

import java.io.IOException;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.assertFalse;

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
        KeycardSigner signer3 = new TestKeycardSigner("secp256k1-pkcs8-2.der", "secp256k1-pub-2.der");

        assertThrows(CothorityCryptoException.class, () -> signer.getPrivate());
        assertThrows(IllegalStateException.class, () -> signer.serialize());

        byte[] msg = "test data to sign".getBytes();
        byte[] sig = signer.sign(msg);
        assertTrue(signer.getIdentity().verify(msg, sig));
        assertTrue(signer2.getIdentity().verify(msg, sig));
        assertFalse(signer3.getIdentity().verify(msg, sig));

        Identity id1 = signer.getIdentity();
        Identity id2 = IdentityFactory.New(id1.toProto());
        Identity id3 = signer3.getIdentity();
        assertTrue(id1.equals(id2));
        assertTrue(id1.verify(msg, sig));
        assertTrue(id2.verify(msg, sig));
        assertFalse(id3.verify(msg, sig));
    }
}
