package ch.epfl.dedis.lib.crypto;

import java.security.SecureRandom;

/**
 * This class is a pair of Ed25519 scalar x and point X such that X = g * x where g is the generator.
 */
public class Ed25519Pair {
    public final Scalar scalar;
    public final Point point;

    /**
     * Construct a pair from a new random source.
     */
    public Ed25519Pair() {
        byte[] seed = new byte[Ed25519.field.getb() / 8];
        new SecureRandom().nextBytes(seed);
        scalar = new Ed25519Scalar(seed);
        point = Ed25519Point.base().mul(scalar);
    }
}
