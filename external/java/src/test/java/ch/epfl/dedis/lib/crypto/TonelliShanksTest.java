package ch.epfl.dedis.lib.crypto;

import org.junit.jupiter.api.Test;

import java.math.BigInteger;

import static org.junit.jupiter.api.Assertions.*;

class TonelliShanksTest {

    @Test
    void modSqrt() {
        // test vectors are taken from https://rosettacode.org/wiki/Tonelli-Shanks_algorithm
        assertEquals(BigInteger.valueOf(7L), TonelliShanks.modSqrt(10L, 13L));
        assertEquals(BigInteger.valueOf(37L), TonelliShanks.modSqrt(56L, 101L));
        assertEquals(BigInteger.valueOf(1632L), TonelliShanks.modSqrt(1030L, 10009L));
        assertNull(TonelliShanks.modSqrt(1032L, 10009L));
        assertEquals(BigInteger.valueOf(378633312L), TonelliShanks.modSqrt(665820697L, 1000000009L));

        assertEquals(new BigInteger("32102985369940620849741983987300038903725266634508"),
                TonelliShanks.modSqrt(new BigInteger("41660815127637347468140745042827704103445750172002"), new BigInteger("100000000000000000000000000000000000000000000000577")));
    }
}