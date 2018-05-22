package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.proto.SkipBlockProto;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;

public class SchnorrSig {
    public Point challenge;
    public Scalar response;

    public SchnorrSig(byte[] msg, Scalar priv) {
        KeyPair kp = new KeyPair();
        challenge = kp.point;

        Point pub = Ed25519Point.base().mul(priv);
        Scalar xh = priv.mul(toHash(challenge, pub, msg));
        response = kp.scalar.add(xh);
    }

    public SchnorrSig(byte[] data) {
        challenge = new Ed25519Point(Arrays.copyOfRange(data, 0, 32));
        response = new Ed25519Scalar(Arrays.copyOfRange(data, 32, 64));
    }

    public boolean verify(byte[] msg, Point pub) {
        Scalar hash = toHash(challenge, pub, msg);
        Point S = Ed25519Point.base().mul(response);
        Point Ah = pub.mul(hash);
        Point RAs = challenge.add(Ah);
        return S.equals(RAs);
    }

    public byte[] toBytes() {
        byte[] buf = new byte[64];
        System.arraycopy(challenge.toBytes(), 0, buf, 0, 32);
        System.arraycopy(response.toBytes(), 0, buf, 32, 32);
        return buf;
    }

    public Scalar toHash(Point challenge, Point pub, byte[] msg) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-512");
            digest.update(challenge.toBytes());
            digest.update(pub.toBytes());
            digest.update(msg);
            byte[] hash = Arrays.copyOfRange(digest.digest(), 0, 64);
            Scalar s = new Ed25519Scalar(hash);
            return s;
        } catch (NoSuchAlgorithmException e) {
            return null;
        }
    }

    public SkipBlockProto.SchnorrSig toProto() {
        SkipBlockProto.SchnorrSig.Builder ss =
                SkipBlockProto.SchnorrSig.newBuilder();
        ss.setChallenge(challenge.toProto());
        ss.setResponse(response.toProto());
        return ss.build();
    }
}
