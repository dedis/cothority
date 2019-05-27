package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.crypto.blake2s.Blake2xsDigest;
import ch.epfl.dedis.lib.crypto.bn256.BN;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * Boneh-Drijvers-Neven (BDN) signature scheme is a modified version of the BLS signature
 * scheme that is robust against rogue public-key attacks.
 *
 * Single signatures are compatible with BLS because only the aggregation of the public
 * keys and the signatures is different.
 */
public class BdnSig {
    private static final int COEF_SIZE = 128 / 8; // 128bits

    private byte[] sig;

    /**
     * Instantiate a BDN signature with the bytes representation
     * @param sig Byte array of the serialized signature
     */
    public BdnSig(byte[] sig) {
        this.sig = Arrays.copyOf(sig, sig.length);
    }

    /**
     * Verify the signature against the message and the aggregated public key. The mask is
     * used to derive the aggregation with only the keys involved in the signature.
     *
     * @param msg   Signed message in bytes
     * @param mask  Mask of the participation of the public keys
     * @return true if the signature matches, false otherwise
     */
    public boolean verify(byte[] msg, Mask mask) {
        Bn256G2Point pubkey = (Bn256G2Point) BdnSig.aggregatePublicKeys(mask);

        // only the way aggregate is computed differs from BLS
        return new BlsSig(this.sig).verify(msg, pubkey);
    }

    /**
     * Make the aggregated public-key of the mask, that is the aggregation of the
     * public key enabled in the mask.
     *
     * @param mask The mask to use
     * @return The point representing the aggregation of the public keys
     */
    static Point aggregatePublicKeys(Mask mask) {
        return BdnSig.aggregatePoints(mask, mask.getPublics());
    }

    /**
     * Make the aggregate of the given points that are enabled in the mask. Note
     * that it must be in the same order and the length must match.
     *
     * @param mask      The mask
     * @param points    The list of points
     * @return The new point representing the aggregation
     */
    static Point aggregatePoints(Mask mask, List<Point> points) {
        List<Point> pubs = mask.getPublics();
        if (pubs.size() != points.size()) {
            throw new IllegalArgumentException("Length of the mask and the list of points does not match");
        }

        List<Scalar> coefs = BdnSig.hashPointToR(pubs);

        Point agg = null;
        for (int i = 0; i < coefs.size(); i++) {
            Scalar c = coefs.get(i);
            Point p = points.get(i);

            if (mask.indexEnabled(i)) {
                p = p.mul(c);
                // R is in the range [1; 2^128] inclusive thus (c+1) * p
                p = p.add(points.get(i));
                if (agg == null) {
                    agg = p;
                } else {
                    agg = agg.add(p);
                }
            }
        }

        return agg;
    }

    /**
     * Generate the list of coefficients from a list of public keys. An eXtendable-Output Function (XOF) is
     * used to generate enough bytes that can be split in integers of 128 bits for each key.
     *
     * @param pubkeys The list of public keys
     * @return A list of scalars ordered by public key
     */
    static List<Scalar> hashPointToR(List<Point> pubkeys) {
        Blake2xsDigest h = new Blake2xsDigest();

        for (Point p : pubkeys) {
            byte[] buff = p.toBytes();
            h.update(buff, 0, buff.length);
        }

        byte[] out = new byte[BdnSig.COEF_SIZE];

        List<Scalar> coefs = new ArrayList<>();
        for (int i = 0; i < pubkeys.size(); i++) {
            h.doOutput(out, 0, out.length);

            byte[] buf = new byte[BdnSig.COEF_SIZE];
            for (int j = 0; j < BdnSig.COEF_SIZE; j++) {
                // BigInteger only takes little endian bytes so we
                // need to reverse the digest
                buf[j] = out[BdnSig.COEF_SIZE-1-j];
            }

            coefs.add(new Bn256Scalar(new BigInteger(1, buf)));
        }

        return coefs;
    }

    /**
     * Generate a signature from a message and a secret key
     *
     * @param msg   Bytes of the message to sign
     * @param x     The secret key
     * @return The signature as a point
     */
    static Point sign(byte[] msg, Scalar x) {
        Point sig = new Bn256G1Point(BN.G1.hashToPoint(msg));
        return sig.mul(x);
    }
}
