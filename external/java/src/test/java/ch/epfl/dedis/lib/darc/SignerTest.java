package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.Test;

import java.io.IOException;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

public class SignerTest {
    @Test
    void instantiateSigner() throws IOException, Exception {
        SignerEd25519 signer = new SignerEd25519();
        Point pub = signer.getPublic();
        Scalar priv = signer.getPrivate();

        assertTrue(Ed25519Point.base().mul(priv).equals(pub));
        assertTrue(pub.equals(SignerFactory.New(signer.serialize()).getPublic()));
    }

    @Test
    void signAndVerify() throws Exception {
        SignerEd25519 signer = new SignerEd25519();
        byte[] msg = "document".getBytes();

        byte[] sig = signer.sign(msg);
        assertTrue(IdentityFactory.New(signer).verify(msg, sig));
    }

    @Test
    void serialize() throws Exception {
        SignerEd25519 signer = new SignerEd25519();
        byte[] buf = signer.serialize();
        Signer signer2 = SignerFactory.New(buf);

        assertTrue(signer.getPrivate().equals(signer2.getPrivate()));
        assertTrue(signer.getPublic().equals(signer2.getPublic()));
    }

    @Test
    void keycard() throws Exception {
        SignerX509EC signer = new TestSignerX509EC("secp256k1-pkcs8.der", "secp256k1-pub.der");
        SignerX509EC signer2 = new TestSignerX509EC("secp256k1-pkcs8.der", "secp256k1-pub.der");

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
