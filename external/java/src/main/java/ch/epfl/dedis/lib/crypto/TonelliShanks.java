package ch.epfl.dedis.lib.crypto;

import java.math.BigInteger;
import java.util.function.BiFunction;
import java.util.function.Function;

public class TonelliShanks {
    private static final BigInteger ZERO = BigInteger.ZERO;
    private static final BigInteger ONE = BigInteger.ONE;
    private static final BigInteger TWO = BigInteger.valueOf(2);
    private static final BigInteger FOUR = BigInteger.valueOf(4);

    /***
     * Computes the square root of n mod p if it exists, otherwise null is returned.
     * In other words, find x such that x^2 = n mod p, where p must be an odd prime.
     * The implementation is adapted from https://rosettacode.org/wiki/Tonelli-Shanks_algorithm#Java
     */
    public static BigInteger modSqrt(BigInteger n, BigInteger p) {
        BiFunction<BigInteger, BigInteger, BigInteger> powModP = (BigInteger a, BigInteger e) -> a.modPow(e, p);
        Function<BigInteger, BigInteger> ls = (BigInteger a) -> powModP.apply(a, p.subtract(ONE).divide(TWO));

        if (!ls.apply(n).equals(ONE)) return null;

        BigInteger q = p.subtract(ONE);
        BigInteger ss = ZERO;
        while (q.and(ONE).equals(ZERO)) {
            ss = ss.add(ONE);
            q = q.shiftRight(1);
        }

        if (ss.equals(ONE)) {
            return powModP.apply(n, p.add(ONE).divide(FOUR));
        }

        BigInteger z = TWO;
        while (!ls.apply(z).equals(p.subtract(ONE))) z = z.add(ONE);
        BigInteger c = powModP.apply(z, q);
        BigInteger r = powModP.apply(n, q.add(ONE).divide(TWO));
        BigInteger t = powModP.apply(n, q);
        BigInteger m = ss;

        while (true) {
            if (t.equals(ONE)) return r;
            BigInteger i = ZERO;
            BigInteger zz = t;
            while (!zz.equals(BigInteger.ONE) && i.compareTo(m.subtract(ONE)) < 0) {
                zz = zz.multiply(zz).mod(p);
                i = i.add(ONE);
            }
            BigInteger b = c;
            BigInteger e = m.subtract(i).subtract(ONE);
            while (e.compareTo(ZERO) > 0) {
                b = b.multiply(b).mod(p);
                e = e.subtract(ONE);
            }
            r = r.multiply(b).mod(p);
            c = b.multiply(b).mod(p);
            t = t.multiply(c).mod(p);
            m = i;
        }
    }

    static BigInteger modSqrt(Long n, Long p) {
        return TonelliShanks.modSqrt(BigInteger.valueOf(n), BigInteger.valueOf(p));
    }
}
