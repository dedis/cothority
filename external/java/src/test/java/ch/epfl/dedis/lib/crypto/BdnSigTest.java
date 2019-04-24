package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.security.SecureRandom;
import java.util.Arrays;
import java.util.List;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.*;

class BdnSigTest {
    private Random rnd = new SecureRandom();

    @Test
    void testHashPointToRReferences() {
        Bn256G2Point p1 = new Bn256G2Point(BigInteger.ONE);
        Bn256G2Point p2 = new Bn256G2Point(BigInteger.valueOf(2));
        Bn256G2Point p3 = new Bn256G2Point(BigInteger.valueOf(3));
        List<Point> pubs = Arrays.asList(p1, p2, p3);

        List<Scalar> coefs = BdnSig.hashPointToR(pubs);

        assertEquals("35b5b395f58aba3b192fb7e1e5f2abd3", Hex.printHexBinary(coefs.get(0).toBytes()).toLowerCase());
        assertEquals("14dcc79d46b09b93075266e47cd4b19e", Hex.printHexBinary(coefs.get(1).toBytes()).toLowerCase());
        assertEquals("933f6013eb3f654f9489d6d45ad04eaf", Hex.printHexBinary(coefs.get(2).toBytes()).toLowerCase());
    }

    @Test
    void testAggregateVerification() throws Exception {
        Bn256Pair kp1 = new Bn256Pair(rnd);
        Bn256Pair kp2 = new Bn256Pair(rnd);
        Bn256Pair kp3 = new Bn256Pair(rnd);
        byte[] msg = "two legs good four legs bad".getBytes();
        List<Point> pubs = Arrays.asList(kp1.point, kp3.point, kp2.point);

        Point sig1 = BdnSig.sign(msg, kp1.scalar);
        Point sig2 = BdnSig.sign(msg, kp2.scalar);
        Mask mask = new Mask(pubs, new byte[]{5});
        Point sig = BdnSig.aggregatePoints(mask, Arrays.asList(sig1, null, sig2));
        BdnSig signature = new BdnSig(sig.toBytes());

        assertTrue(signature.verify(msg, mask));

        byte[] wrongMsg = "abc".getBytes();
        assertFalse(signature.verify(wrongMsg, mask));

        Mask wrongMask = new Mask(pubs, new byte[]{7});
        assertFalse(signature.verify(msg, wrongMask));
    }
}
