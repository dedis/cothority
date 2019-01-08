package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.bn256.BN;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.Test;

import java.security.SecureRandom;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

class Bn256G1PointTest {
    private Random rnd = new SecureRandom();
    @Test
    void testConstructors() {
        assertThrows(CothorityCryptoException.class, () -> new Bn256G1Point(""));
        assertThrows(CothorityCryptoException.class, () -> new Bn256G1Point(new byte[0]));
    }

    @Test
    void add() {
        BN.PairG1 g1pair = BN.G1.rand(rnd);
        BN.PairG1 g1pair2 = BN.G1.rand(rnd);
        Bn256G1Point g1 = new Bn256G1Point(g1pair.getPoint());
        Bn256G1Point g12 = new Bn256G1Point(g1pair2.getPoint());
        assertEquals(g1.add(g12).add(g12.negate()), g1);
    }

    @Test
    void mul() {
        BN.PairG1 g1pair = BN.G1.rand(rnd);
        BN.PairG1 dummy = BN.G1.rand(rnd);
        Bn256G1Point g1 = new Bn256G1Point(g1pair.getPoint());
        Bn256Scalar scalar = new Bn256Scalar(dummy.getScalar());
        assertEquals(g1.mul(scalar).mul(scalar.invert()), g1);
    }

    @Test
    void marshal() throws Exception {
        BN.PairG1 g1pair = BN.G1.rand(rnd);
        Bn256G1Point g1 = new Bn256G1Point(g1pair.getPoint());
        assertEquals(new Bn256G1Point(g1.toBytes()), g1);
        assertEquals(new Bn256G1Point(g1.toProto().toByteArray()), g1);
    }
}
