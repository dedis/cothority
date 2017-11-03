package ch.epfl.dedis.lib.crypto;

import java.util.Random;

public class KeyPair {
    public Scalar Scalar;
    public Point Point;

    public KeyPair() {
        byte[] seed = new byte[Ed25519.field.getb() / 8];
        new Random().nextBytes(seed);
        Scalar = new Scalar(seed);
        Point = Scalar.scalarMult(null);
    }
}
