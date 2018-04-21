package ch.epfl.dedis.lib.crypto;

import java.util.Random;

public class KeyPair {
    public Scalar scalar;
    public Point Point;

    public KeyPair() {
        byte[] seed = new byte[Ed25519.field.getb() / 8];
        new Random().nextBytes(seed);
        scalar = new Ed25519Scalar(seed);
        Point = scalar.scalarMult(null);
    }
}
