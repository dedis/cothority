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
        kp.Scalar = new Scalar("983f5db380f69c1f58164fa8af1d478842559854989d9f559ad8ea544d12e8");
        kp.Point = kp.Scalar.scalarMult(null);
        challenge = kp.Point;

        Point pub = priv.scalarMult(null);
        Scalar xh = priv.mul(toHash(challenge, pub, msg));
        response = kp.Scalar.add(xh);
    }

    public SchnorrSig(byte[] data) {
        challenge = new Point(Arrays.copyOfRange(data, 0, 32));
        response = new Scalar(Arrays.copyOfRange(data, 32, 64));
    }

    public boolean Verify(byte[] msg, Point pub) {
        Scalar hash = toHash(challenge, pub, msg);
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

    public Scalar toHash(Point challenge, Point pub, byte[] msg) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-512");
            digest.update(challenge.toBytes());
            digest.update(pub.toBytes());
            digest.update(msg);
            byte[] hash = Arrays.copyOfRange(digest.digest(), 0, 64);
            Scalar s = new Scalar(hash);
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
