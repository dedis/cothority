package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.bn256.BN;

import java.security.SecureRandom;
import java.util.Random;

/**
 * This class is a pair of BN256 scalar x and point X such that X = g * x where g is the generator.
 */
public class Bn256Pair {
    public final Scalar scalar;
    public final Point point;

    /**
     * Construct a pair from a new random source.
     */
    public Bn256Pair() {
        this(new SecureRandom());
    }

    /**
     * Construct a pair from an existing random source.
     *
     * @param rnd is the random source.
     */
    public Bn256Pair(Random rnd) {
        BN.PairG2 pair = BN.G2.rand(rnd);
        this.point = new Bn256G2Point(pair.getPoint());
        this.scalar = new Bn256Scalar(pair.getScalar());
    }
}
