import BN from 'bn.js';
import { BNType, zeroBN, oneBN } from '../constants';

const ZERO = zeroBN;
const ONE = oneBN;
const TWO = new BN(2);
const FOUR = new BN(4);

function powMod(a: BN, e: BN, p: BN): BN {
    return a.toRed(BN.red(p)).redPow(e).fromRed();
}

function ls(a: BN, p: BN): BN {
    return powMod(a, p.sub(ONE).div(TWO), p);
}

/**
 * Computes the square root of n mod p if it exists, otherwise null is returned.
 * In other words, find x such that x^2 = n mod p, where p must be an odd prime.
 * The implementation is adapted from https://rosettacode.org/wiki/Tonelli-Shanks_algorithm#Java
 * to reflect the Java library.
 */
export function modSqrt(n: BNType, p: BNType): BN {
    n = new BN(n);
    p = new BN(p);

    if (!ls(n, p).eq(ONE)) {
        return null;
    }

    let q = p.sub(ONE);
    let ss = ZERO;
    while (q.and(ONE).eq(ZERO)) {
        ss = ss.add(ONE);
        q = q.shrn(1);
    }

    if (ss.eq(ONE)) {
        return powMod(n, p.add(ONE).div(FOUR), p);
    }

    let z = TWO;
    while (!ls(z, p).eq(p.sub(ONE))) {
        z = z.add(ONE);
    }

    let c = powMod(z, q, p);
    let r = powMod(n, q.add(ONE).div(TWO), p);
    let t = powMod(n, q, p);
    let m = ss;

    for (;;) {
        if (t.eq(ONE)) {
            return r;
        }

        let i = ZERO;
        let zz = t;
        while (!zz.eq(ONE) && i.cmp(m.sub(ONE)) < 0) {
            zz = zz.mul(zz).mod(p);
            i = i.add(ONE);
        }

        let b = c;
        let e = m.sub(i).sub(ONE);
        while (e.cmp(ZERO) > 0) {
            b = b.mul(b).mod(p);
            e = e.sub(ONE);
        }

        r = r.mul(b).mod(p);
        c = b.mul(b).mod(p);
        t = t.mul(c).mod(p);
        m = i;
    }
}
