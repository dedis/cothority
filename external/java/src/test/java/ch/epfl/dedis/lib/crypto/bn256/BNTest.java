package ch.epfl.dedis.lib.crypto.bn256;

import ch.epfl.dedis.lib.Hex;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

import java.math.BigInteger;
import java.security.SecureRandom;
import java.util.Arrays;
import java.util.Random;

class BNTest {

    static boolean isZero(BigInteger a) {
        return a.mod(Constants.p).equals(BigInteger.ZERO);
    }

    static boolean isOne(BigInteger a) {
        return a.mod(Constants.p).equals(BigInteger.ONE);
    }

    @Test
    void gfp2Invert() {
        GFp2 a = new GFp2(new BigInteger("23423492374"), new BigInteger("12934872398472394827398470"));

        GFp2 inv = new GFp2();
        inv.invert(a);

        GFp2 b = new GFp2();
        b.mul(inv, a);
        assertFalse(!isZero(b.x) || !isOne(b.y), "bad result for a^-1*a");
    }

    @Test
    void gfp6Invert() {
        GFp6 a = new GFp6();
        a.x.x = new BigInteger("239487238491");
        a.x.y = new BigInteger("2356249827341");
        a.y.x = new BigInteger("082659782");
        a.y.y = new BigInteger("182703523765");
        a.z.x = new BigInteger("978236549263");
        a.z.y = new BigInteger("64893242");

        GFp6 inv = new GFp6();
        inv.invert(a);

        GFp6 b = new GFp6();
        b.mul(inv, a);

        assertFalse(
                !isZero(b.x.x) || !isZero(b.x.y) ||
                        !isZero(b.y.x) || !isZero(b.y.y) ||
                        !isZero(b.z.x) || !isOne(b.z.y),
                "bad result for a^-1*a");
    }

    @Test
    void gf12Invert() {
        GFp12 a = new GFp12();
        a.x.x.x = new BigInteger("239846234862342323958623");
        a.x.x.y = new BigInteger("2359862352529835623");
        a.x.y.x = new BigInteger("928836523");
        a.x.y.y = new BigInteger("9856234");
        a.x.z.x = new BigInteger("235635286");
        a.x.z.y = new BigInteger("5628392833");
        a.y.x.x = new BigInteger("252936598265329856238956532167968");
        a.y.x.y = new BigInteger("23596239865236954178968");
        a.y.y.x = new BigInteger("95421692834");
        a.y.y.y = new BigInteger("236548");
        a.y.z.x = new BigInteger("924523");
        a.y.z.y = new BigInteger("12954623");

        GFp12 inv = new GFp12();
        inv.invert(a);

        GFp12 b = new GFp12();
        b.mul(inv, a);
        assertFalse(
                !isZero(b.x.x.x) || !isZero(b.x.x.y) ||
                        !isZero(b.x.y.x) || !isZero(b.x.y.y) ||
                        !isZero(b.x.z.x) || !isZero(b.x.z.y) ||
                        !isZero(b.y.x.x) || !isZero(b.y.x.y) ||
                        !isZero(b.y.y.x) || !isZero(b.y.y.y) ||
                        !isZero(b.y.z.x) || !isOne(b.y.z.y),
                "bad result for a^-1*a");
    }

    @Test
    void curveImpl() {
        CurvePoint g = new CurvePoint();
        g.x = BigInteger.ONE;
        g.y = new BigInteger("-2");
        g.z = BigInteger.ONE;
        g.t = BigInteger.ZERO;

        BigInteger x = new BigInteger("32498273234");
        CurvePoint X = new CurvePoint().mul(g, x);

        BigInteger y = new BigInteger("98732423523");
        CurvePoint Y = new CurvePoint().mul(g, y);

        CurvePoint s1 = new CurvePoint().mul(X, y).makeAffine();
        CurvePoint s2 = new CurvePoint().mul(Y, x).makeAffine();

        assertFalse(s1.x.compareTo(s2.x) != 0 || s2.x.compareTo(s1.x) != 0, "DH points don't match");
    }

    @Test
    void orderG1() {
        BN.G1 g = new BN.G1().scalarBaseMul(Constants.order);
        assertFalse(!g.p.isInfinity(), "G1 has incorrect order");

        BN.G1 one = new BN.G1().scalarBaseMul(BigInteger.ONE);
        g.add(g, one);
        g.p.makeAffine();
        assertFalse(g.p.x.compareTo(one.p.x) != 0 || g.p.y.compareTo(one.p.y) != 0, "1+0 != 1 in G!");
    }

    @Test
    void orderG2() {
        BN.G2 g = new BN.G2().scalarBaseMul(Constants.order);
        assertFalse(!g.p.isInfinity(), "G2 has incorrect order");

        BN.G2 one = new BN.G2().scalarBaseMul(BigInteger.ONE);
        g.add(g, one);
        g.p.makeAffine();
        assertFalse(g.p.x.x.compareTo(one.p.x.x) != 0 || g.p.x.y.compareTo(one.p.x.y) != 0 ||
                g.p.y.x.compareTo(one.p.y.x) != 0 || g.p.y.y.compareTo(one.p.y.y) != 0,
                "1+0 != 1 in G2");
    }

    @Test
    void orderGT() {
        BN.GT gt = BN.pair(new BN.G1(CurvePoint.curveGen), new BN.G2(TwistPoint.twistGen));
        BN.GT g = new BN.GT().scalarMul(gt, Constants.order);
        assertFalse(!g.p.isOne(), "GT has incorrect order");
    }

    @Test
    void bilinearity() {
        Random rnd = new SecureRandom();
        for (int i = 0; i < 2; i++) {
            BN.PairG1 pairG1 = BN.G1.rand(rnd);
            BN.PairG2 pairG2 = BN.G2.rand(rnd);
            BigInteger a = pairG1.getScalar();
            BN.G1 p1 = pairG1.getPoint();
            BigInteger b = pairG2.getScalar();
            BN.G2 p2 = pairG2.getPoint();
            BN.GT e1 = BN.pair(p1, p2);

            BN.GT e2 = BN.pair(new BN.G1(CurvePoint.curveGen), new BN.G2(TwistPoint.twistGen));
            e2.scalarMul(e2, a);
            e2.scalarMul(e2, b);

            BN.GT minusE2 = new BN.GT().neg(e2);
            e1.add(e1, minusE2);

            assertFalse(!e1.p.isOne(), "bad pairing result: " + e1.toString());
        }
    }

    /**
     * This test is to check our implementation against the golang/crypto/bn256 implementation.
     */
    @Test
    void bilinearityReference() {
        BigInteger a = new BigInteger("12345");
        BN.G1 p1 = new BN.G1(a);
        BigInteger b = new BigInteger("67890");
        BN.G2 p2 = new BN.G2(b);
        BN.GT e1 = BN.pair(p1, p2);

        BN.GT e2 = BN.pair(new BN.G1(CurvePoint.curveGen), new BN.G2(TwistPoint.twistGen));
        e2.scalarMul(e2, a);
        e2.scalarMul(e2, b);

        assertTrue(Arrays.equals(e1.marshal(), Hex.parseHexBinary("2c1660475bb9afe5a514d2ee8a2ff66e449024b0872a30e8d75a297cf6c82a0c79919ee0dd5618ecc6e89042b6ae7f74c9593b74e6e7ae344553af4578c0c6834e9421c990eff3660c4ca488a092eb9434b3c4a25f3585425b409064cc446748357c04ae026baee936e32d3a32489f1d9db346791b88641ef3ef5f2dcf3cebd423e23465a2c96e600ea83eb9cf3c5ffb50beb926560a569ee80d52e165ddcb94817cf8d696d2def79933dc0374ad1ac09b3f4834e17723374babde2f492473d41ca6856b6176795ba662de2f4a1208f1c3b3c5d4138929fa778d2aa2fcec7951457e039854ce6e3ebfcd75f317732abccfa233b5c6443d296bfaa5e7d6398c8d31db50c7ee4fe3ab79f311180711605a3f09f148edc5ffaf00b8bdc90a38702c301cd778cdbab48e375a783283759608a68bc933414f03f04083c12596b0d8ce798e7b670980dfe60a9fdbac4554455b4628e043696210da773b153433f0957b3245a9ba5b23ac3afecd786e692553f2ec42f7a2ff7a6bd4f204c4bf5d708831")));

        BN.GT minusE2 = new BN.GT().neg(e2);
        e1.add(e1, minusE2);

        assertFalse(!e1.p.isOne(), "bad pairing result: " + e1.toString());
    }

    @Test
    void g1Marshal() {
        BN.G1 g = new BN.G1().scalarBaseMul(BigInteger.ONE);
        byte[] from = g.marshal();
        assertNotNull(new BN.G1().unmarshal(from));

        g.scalarBaseMul(Constants.order);
        from = g.marshal();
        BN.G1 g2 = new BN.G1().unmarshal(from);
        assertNotNull(g2);
        assertFalse(!g2.p.isInfinity(), "inf marshaled incorrectly");
    }

    @Test
    void g2Marshal() {
        BN.G2 g = new BN.G2().scalarBaseMul(BigInteger.ONE);
        byte[] from = g.marshal();
        assertNotNull(new BN.G2().unmarshal(from));

        g.scalarBaseMul(Constants.order);
        from = g.marshal();
        BN.G2 g2 = new BN.G2().unmarshal(from);
        assertNotNull(g2);
        assertFalse(!g2.p.isInfinity(), "inf marshaled incorrectly");
    }

    @Test
    void g1Identity() {
        BN.G1 g = new BN.G1().scalarBaseMul(BigInteger.ZERO);
        assertFalse(!g.p.isInfinity(), "failure");
    }

    @Test
    void g2Identity() {
        BN.G2 g = new BN.G2().scalarBaseMul(BigInteger.ZERO);
        assertFalse(!g.p.isInfinity(), "failure");
    }

    @Test
    void tripartiteDiffieHellman() {
        Random rnd = new SecureRandom();
        BigInteger a = BN.randPosBigInt(rnd, Constants.p);
        BigInteger b = BN.randPosBigInt(rnd, Constants.p);
        BigInteger c = BN.randPosBigInt(rnd, Constants.p);

        BN.G1 pa = new BN.G1().unmarshal(new BN.G1().scalarBaseMul(a).marshal());
        BN.G2 qa = new BN.G2().unmarshal(new BN.G2().scalarBaseMul(a).marshal());
        BN.G1 pb = new BN.G1().unmarshal(new BN.G1().scalarBaseMul(b).marshal());
        BN.G2 qb = new BN.G2().unmarshal(new BN.G2().scalarBaseMul(b).marshal());
        BN.G1 pc = new BN.G1().unmarshal(new BN.G1().scalarBaseMul(c).marshal());
        BN.G2 qc = new BN.G2().unmarshal(new BN.G2().scalarBaseMul(c).marshal());

        BN.GT k1 = BN.pair(pb, qc);
        k1.scalarMul(k1, a);
        byte[] k1Bytes = k1.marshal();
        assertTrue(Arrays.equals(new BN.GT().unmarshal(k1Bytes).marshal(), k1Bytes), "failed to unmarshal GT k1");

        BN.GT k2 = BN.pair(pc, qa);
        k2.scalarMul(k2, b);
        byte[] k2Bytes = k2.marshal();
        assertTrue(Arrays.equals(new BN.GT().unmarshal(k2Bytes).marshal(), k2Bytes), "failed to unmarshal GT k2");

        BN.GT k3 = BN.pair(pa, qb);
        k3.scalarMul(k3, c);
        byte[] k3Bytes = k3.marshal();
        assertTrue(Arrays.equals(new BN.GT().unmarshal(k3Bytes).marshal(), k3Bytes), "failed to unmarshal GT k3");

        assertFalse(!Arrays.equals(k1Bytes, k2Bytes) || !Arrays.equals(k2Bytes, k3Bytes), "keys didn't agree");
    }

    @Test
    void benchmarkPairing() {
        for (int i = 0; i < 10; i++) {
            BN.pair(new BN.G1(CurvePoint.curveGen), new BN.G2(TwistPoint.twistGen));
        }
    }
}