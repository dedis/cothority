package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

public class DarcSignatureTest {
    @Test
    void testSigning() throws Exception{
        Ed25519Signer signer = new Ed25519Signer();
        Ed25519Signer signer2 = new Ed25519Signer();
        byte[] msg = "document".getBytes();
        Darc darc = new Darc(signer, Arrays.asList(signer), null);
        Darc darc2 = new Darc(signer2, null, null);
        List<Darc> dpath = new ArrayList<>();
        dpath.add(darc);
        SignaturePath path = new SignaturePath(dpath, signer, SignaturePath.OWNER);

        DarcSignature sig = new DarcSignature(msg, path, signer);
        assertTrue(sig.verify(msg, darc));
        assertFalse(sig.verify(msg, darc2));
    }

    @Test
    void serialization() throws Exception{
        Ed25519Signer signer = new Ed25519Signer();
        byte[] msg = "document".getBytes();
        Darc darc = new Darc(signer, null, null);
        SignaturePath path = new SignaturePath(darc, signer, SignaturePath.OWNER);

        DarcSignature sig = new DarcSignature(msg, path, signer);
        DarcProto.Signature proto = sig.toProto();
        DarcSignature sig2 = new DarcSignature(proto);

        assertTrue(sig.equals(sig2));
    }
}
