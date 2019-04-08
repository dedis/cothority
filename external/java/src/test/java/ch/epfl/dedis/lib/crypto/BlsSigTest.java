package ch.epfl.dedis.lib.crypto;

import org.junit.jupiter.api.Test;

import java.security.SecureRandom;
import java.util.Arrays;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

class BlsSigTest {
    private Random rnd = new SecureRandom();
    @Test
    void fail() {
        Bn256Pair pair = new Bn256Pair(rnd);
        Bn256Pair pair2 = new Bn256Pair(rnd);
        byte[] msg = "two legs good four legs bad".getBytes();

        BlsSig badSig = new BlsSig("wrong signature".getBytes());
        assertFalse(badSig.verify(msg, (Bn256G2Point)pair.point));

        badSig = new BlsSig(msg, pair2.scalar);
        assertFalse(badSig.verify(msg, (Bn256G2Point)pair.point));
    }

    @Test
    void ok() {
        Bn256Pair pair = new Bn256Pair(rnd);
        byte[] msg = "two legs good four legs better".getBytes();
        BlsSig goodSig = new BlsSig(msg, pair.scalar);
        assertTrue(goodSig.verify(msg, (Bn256G2Point) pair.point));
    }

    @Test
    void random() {
        for (int i = 0; i < 10; i++) {
            byte[] msg = new byte[256];
            rnd.nextBytes(msg);

            Bn256Pair pair = new Bn256Pair(rnd);
            BlsSig goodSig = new BlsSig(msg, pair.scalar);
            assertTrue(goodSig.verify(msg, (Bn256G2Point) pair.point));

            byte[] badMsg = Arrays.copyOfRange(msg, 0, 255);
            assertFalse(goodSig.verify(badMsg, (Bn256G2Point) pair.point));
        }
    }
}