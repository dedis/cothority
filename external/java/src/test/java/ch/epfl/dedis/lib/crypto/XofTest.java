package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import org.bouncycastle.crypto.digests.SHAKEDigest;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class XofTest {
    @Test
    void compatibility() {
        SHAKEDigest d = new SHAKEDigest(256);

        byte[] seed = "helloworld".getBytes();
        d.update(seed, 0, seed.length);

        byte[] out1 = new byte[64];
        d.doOutput(out1, 0, 64);
        assertArrayEquals(Hex.parseHexBinary("0599df850188c1933b38dc74b7e6972bc054234f01cd7f9e8e2e8cc40acb149d894d9b3d8149cafe7ff89526576c7d8626424a83c82522d4b8120fceca7f7319"), out1);

        byte[] out2 = new byte[64];
        d.doOutput(out2, 0, 64);
        assertArrayEquals(Hex.parseHexBinary("c33eae5774174971b4b3470b3372014d6d3fd019385ffad099ac444860dfeb3c2ce6d2629ef0570a132a7db23326c825e981735fa7d61fca2bbcbc69dae37fcd"), out2);
    }
}
