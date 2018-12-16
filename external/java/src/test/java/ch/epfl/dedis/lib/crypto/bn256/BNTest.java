package ch.epfl.dedis.lib.crypto.bn256;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

import java.math.BigInteger;
import java.security.SecureRandom;
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
            BigInteger a = pairG1.k;
            BN.G1 p1 = pairG1.p;
            BigInteger b = pairG2.k;
            BN.G2 p2 = pairG2.p;
            BN.GT e1 = BN.pair(p1, p2);

            BN.GT e2 = BN.pair(new BN.G1(CurvePoint.curveGen), new BN.G2(TwistPoint.twistGen));
            e2.scalarMul(e2, a);
            e2.scalarMul(e2, b);

            BN.GT minusE2 = new BN.GT().neg(e2);
            e1.add(e1, minusE2);

            assertFalse(!e1.p.isOne(), "bad pairing result: " + e1.toString());
        }
    }

    // TODO marshal test

    @Test
    void g1Identity() {
        BN.G1 g = new BN.G1().scalarBaseMul(BigInteger.ZERO);
        assertFalse(!g.p.isInfinity(), "failure");
    }

    @Test
    void g2Identyt() {
        BN.G2 g = new BN.G2().scalarBaseMul(BigInteger.ZERO);
        assertFalse(!g.p.isInfinity(), "failure");
    }

    @Test
    void tripartiteDiffieHellman() {
        // TODO
    }
}