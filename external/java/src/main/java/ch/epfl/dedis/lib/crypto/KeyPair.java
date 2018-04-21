package ch.epfl.dedis.lib.crypto;

import java.util.Random;

public class KeyPair {
    public Ed25519Scalar Ed25519Scalar;
    public Point Point;

    public KeyPair() {
        byte[] seed = new byte[Ed25519.field.getb() / 8];
        new Random().nextBytes(seed);
        Ed25519Scalar = new Ed25519Scalar(seed);
        Point = Ed25519Scalar.scalarMult(null);
    }
}
