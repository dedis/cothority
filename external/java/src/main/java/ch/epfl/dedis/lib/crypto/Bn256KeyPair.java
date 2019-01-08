package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.bn256.BN;

import java.security.SecureRandom;
import java.util.Random;

public class Bn256KeyPair {
    public Scalar scalar;
    public Point point;

    public Bn256KeyPair() {
        this(new SecureRandom());
    }

    public Bn256KeyPair(Random rnd) {
        BN.PairG2 pair = BN.G2.rand(rnd);
        this.point = new Bn256G2Point(pair.getPoint());
        this.scalar = new Bn256Scalar(pair.getScalar());
    }
}
