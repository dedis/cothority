package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.bn256.BN;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.Test;

import java.security.SecureRandom;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

class Bn256G2PointTest {
    private Random rnd = new SecureRandom();
    @Test
    void testConstructors() {
        assertThrows(CothorityCryptoException.class, () -> new Bn256G2Point(""));
        assertThrows(CothorityCryptoException.class, () -> new Bn256G2Point(new byte[0]));
    }

    @Test
    void add() {
        BN.PairG2 g2pair = BN.G2.rand(rnd);
        BN.PairG2 g2pair2 = BN.G2.rand(rnd);
        Bn256G2Point g2 = new Bn256G2Point(g2pair.getPoint());
        Bn256G2Point g22 = new Bn256G2Point(g2pair2.getPoint());
        assertEquals(g2.add(g22).add(g22.negate()), g2);
    }

    @Test
    void mul() {
        BN.PairG2 g2pair = BN.G2.rand(rnd);
        BN.PairG2 dummy = BN.G2.rand(rnd);
        Bn256G2Point g2 = new Bn256G2Point(g2pair.getPoint());
        Bn256Scalar scalar = new Bn256Scalar(dummy.getScalar());
        assertEquals(g2.mul(scalar).mul(scalar.invert()), g2);
    }

    @Test
    void marshal() throws Exception {
        BN.PairG2 g2pair = BN.G2.rand(rnd);
        Bn256G2Point g2 = new Bn256G2Point(g2pair.getPoint());
        assertEquals(new Bn256G2Point(g2.toBytes()), g2);
        assertEquals(new Bn256G2Point(g2.toProto().toByteArray()), g2);
    }
}
