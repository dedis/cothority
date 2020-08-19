package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.bn256.BN;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.*;

class MaskTest {

    private List<Point> publics;
    private Random rnd = new SecureRandom();
    private int n = 9;

    MaskTest() {
        publics = new ArrayList<>();
        for (int i = 0; i < n; i++) {
            publics.add(new Bn256G2Point(BN.G2.rand(rnd).getPoint()));
        }
    }

    @Test
    void len() throws Exception {
        Mask mask = new Mask(publics, new byte[]{0,0});
        assertEquals(2, mask.len(), "invalid size");
    }

    @Test
    void getAggregate() throws Exception {
        Mask mask = new Mask(publics, new byte[]{0,0});
        assertTrue(mask.getAggregate().isZero(),"aggregate should be zero when the mask is not set");

        mask = new Mask(publics, new byte[]{1, 0});
        assertFalse(mask.getAggregate().isZero(),"aggregate should not be zero");
        assertEquals(publics.get(0), mask.getAggregate());

        mask = new Mask(publics, new byte[]{(byte)255, (byte)255});
        assertFalse(mask.getAggregate().isZero(),"aggregate should not be zero");
    }

    @Test
    void indexEnabled() throws Exception {
        Mask mask = new Mask(publics, new byte[]{(byte)255, (byte)255});
        for (int i = 0; i < this.publics.size(); i++) {
            assertTrue(mask.indexEnabled(i), "not enabled: " + i);
        }
        mask = new Mask(publics, new byte[]{0,0});
        for (int i = 0; i < this.publics.size(); i++) {
            assertFalse(mask.indexEnabled(i), "should be enabled: " + i);
        }
    }

    @Test
    void keyEnabled() throws Exception {
        Mask mask = new Mask(publics, new byte[]{(byte)255, (byte)255});
        for (Point p : this.publics) {
            assertTrue(mask.keyEnabled(p), "should be enabled");
        }

        mask = new Mask(publics, new byte[]{0, 0});
        for (Point p : this.publics) {
            assertFalse(mask.keyEnabled(p), "should not be enabled");
        }
    }

    @Test
    void countEnabled() throws Exception {
        // enable all
        Mask mask = new Mask(publics, new byte[]{(byte)255, (byte)255});
        assertEquals(n, mask.countEnabled());

        // disable one
        mask = new Mask(publics, new byte[]{(byte)254, (byte)255});
        assertEquals(n-1, mask.countEnabled());

        // disable another one
        mask = new Mask(publics, new byte[]{(byte)255, (byte)0});
        assertEquals(n-1, mask.countEnabled());

        // disable all
        mask = new Mask(publics, new byte[]{0,0});
        assertEquals(0, mask.countEnabled());
    }

    @Test
    void countTotal() throws Exception {
        Mask mask = new Mask(publics, new byte[]{0,0});
        assertEquals(mask.countTotal(), n);


        mask = new Mask(publics.subList(0, n-1), new byte[]{0});
        assertEquals(mask.countTotal(), n-1);
    }

    @Test
    void verifySignature() throws Exception {
        byte[] msg = "hello".getBytes();
        Random rnd = new SecureRandom();

        List<Bn256Pair> pairs = new ArrayList<>();
        for (int i = 0; i < n; i++) {
            pairs.add(new Bn256Pair(rnd));
        }

        List<Point> publics = new ArrayList<>();
        for (int i = 0; i < n; i++) {
            publics.add(pairs.get(i).point);
        }

        Point aggrSig = new Bn256G1Point(BigInteger.ONE);
        aggrSig = aggrSig.getZero();
        for (Bn256Pair p : pairs) {
            aggrSig = aggrSig.add(new Bn256G1Point(new BlsSig(msg, p.scalar).getSig()));
        }

        // pass with the right aggregate
        Mask mask = new Mask(publics, new byte[]{(byte)255, (byte)255});
        assertTrue(new BlsSig(aggrSig.toBytes()).verify(msg, (Bn256G2Point)mask.getAggregate()));

        // fail with the wrong aggregate
        mask = new Mask(publics, new byte[]{(byte)254, (byte)255});
        assertFalse(new BlsSig(aggrSig.toBytes()).verify(msg, (Bn256G2Point)mask.getAggregate()));
    }
}
