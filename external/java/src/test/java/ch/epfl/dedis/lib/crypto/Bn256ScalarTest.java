package ch.epfl.dedis.lib.crypto;

import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.security.SecureRandom;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.assertTrue;

class Bn256ScalarTest {
    private Random rnd = new SecureRandom();

    @Test
    void add() {
        Bn256Pair pair = new Bn256Pair(rnd);
        Bn256Pair pair2 = new Bn256Pair(rnd);
        assertTrue(pair.scalar.add(pair2.scalar).sub(pair2.scalar).equals(pair.scalar));
        assertTrue(pair.scalar.add(pair2.scalar).add(pair2.scalar.negate()).equals(pair.scalar));
    }

    @Test
    void mul() {
        Bn256Pair pair = new Bn256Pair(rnd);
        assertTrue(pair.scalar.mul(pair.scalar.invert()).reduce().equals(new Bn256Scalar(BigInteger.ONE)));
    }

    @Test
    void marshal() {
        Bn256Pair pair = new Bn256Pair(rnd);
        assertTrue(new Bn256Scalar(pair.scalar.toBytes()).equals(pair.scalar));
    }
}
