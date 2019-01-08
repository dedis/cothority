package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.bn256.BN;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

import java.math.BigInteger;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;

public class BlsSig {
    private byte[] sig;

    BlsSig(byte[] sig) {
        this.sig = sig;
    }

    /**
     * Verify checks the given BLS signature S on the message m using the public
     * key X by verifying that the equality e(H(m), X) == e(H(m), x*B2) ==
     * e(x*H(m), B2) == e(S, B2) holds where e is the pairing operation and B2 is
     * the base point from curve G2.
     *
     * @param msg the signed message .
     * @param X the public key.
     * @return true if the verification is successful.
     */
    public boolean verify(byte[] msg, Bn256G2Point X) {
        Bn256G1Point HM = hashToPoint(msg);
        BN.GT left = HM.pair(X);
        try {
            Bn256G1Point s = new Bn256G1Point(sig);
            if (s.g1 == null) {
                return false;
            }
            BN.GT right = s.pair(new Bn256G2Point(BigInteger.ONE));
            return left.equals(right);
        } catch (CothorityCryptoException e) {
            return false;
        }
    }

    /**
     * Sign creates a BLS signature S = x * H(m) on a message m using the private
     * key x. The signature S is a point on curve G1.
     *
     * @param x is the secret.
     * @param msg is the message to be signed.
     * @return the signature.
     */
    public static BlsSig sign(Scalar x, byte[] msg) {
        Bn256G1Point HM = hashToPoint(msg);
        Point xHM = HM.mul(x);
        return new BlsSig(xHM.toBytes());
    }

    private static Bn256G1Point hashToPoint(byte[] msg) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(msg);
            BigInteger scalar = new BigInteger(1, digest.digest());
            return new Bn256G1Point(scalar);
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }
}
