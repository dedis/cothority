package ch.epfl.dedis.lib.crypto;

import org.junit.jupiter.api.Test;

import java.security.SecureRandom;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.*;

class BlsSigTest {
    private Random rnd = new SecureRandom();
    @Test
    void fail() {
        Bn256KeyPair pair = new Bn256KeyPair(rnd);
        Bn256KeyPair pair2 = new Bn256KeyPair(rnd);
        byte[] msg = "two legs good four legs bad".getBytes();

        BlsSig badSig = new BlsSig("wrong signature".getBytes());
        assertFalse(badSig.verify(msg, (Bn256G2Point)pair.point));

        badSig = BlsSig.sign(pair2.scalar, msg);
        assertFalse(badSig.verify(msg, (Bn256G2Point)pair.point));
    }

    @Test
    void ok() {
        Bn256KeyPair pair = new Bn256KeyPair(rnd);
        byte[] msg = "two legs good four legs better".getBytes();
        BlsSig goodSig = BlsSig.sign(pair.scalar, msg);
        assertTrue(goodSig.verify(msg, (Bn256G2Point) pair.point));
    }
}