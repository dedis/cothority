package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.proto.SkipBlockProto;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;

public class SchnorrSig {
    public Point challenge;
    public Ed25519Scalar response;

    public SchnorrSig(byte[] msg, Ed25519Scalar priv) {
        KeyPair kp = new KeyPair();
        challenge = kp.Point;

        Point pub = priv.scalarMult(null);
        Ed25519Scalar xh = priv.mul(toHash(challenge, pub, msg));
        response = kp.Ed25519Scalar.add(xh);
    }

    public SchnorrSig(byte[] data) {
        challenge = new Point(Arrays.copyOfRange(data, 0, 32));
        response = new Ed25519Scalar(Arrays.copyOfRange(data, 32, 64));
    }

    public boolean verify(byte[] msg, Point pub) {
        Ed25519Scalar hash = toHash(challenge, pub, msg);
        Point S = response.scalarMult(null);
        Point Ah = pub.scalarMult(hash);
        Point RAs = challenge.add(Ah);
        return S.equals(RAs);
    }

    public byte[] toBytes() {
        byte[] buf = new byte[64];
        System.arraycopy(challenge.toBytes(), 0, buf, 0, 32);
        System.arraycopy(response.toBytes(), 0, buf, 32, 32);
        return buf;
    }

    public Ed25519Scalar toHash(Point challenge, Point pub, byte[] msg) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-512");
            digest.update(challenge.toBytes());
            digest.update(pub.toBytes());
            digest.update(msg);
            byte[] hash = Arrays.copyOfRange(digest.digest(), 0, 64);
            Ed25519Scalar s = new Ed25519Scalar(hash);
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
