package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.OCSProto;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * dedis/lib
 * DecryptKey.java
 * Purpose: Does the onchain-secrets algorithm to retrieve the symmetric from the cothority.
 */

public class DecryptKey {
    public List<Point> Cs;
    public Point XhatEnc;
    public Point X;

    public DecryptKey() {
        Cs = new ArrayList<>();
    }

    public DecryptKey(OCSProto.DecryptKeyReply reply, Point X) {
        this();
        reply.getCsList().forEach(C -> Cs.add(new Point(C)));
        XhatEnc = new Point(reply.getXhatenc());
        this.X = X;
    }

    public byte[] getKeyMaterial(OCSProto.Write write, Scalar reader) throws CothorityCryptoException {
        List<Point> Cs = new ArrayList<>();
        write.getCsList().forEach(cs -> Cs.add(new Point(cs)));

        // Use our private key to decrypt the re-encryption key and use it
        // to recover the symmetric key.
        Scalar xc = reader.reduce();
        Scalar xcInv = xc.negate();
        Point XhatDec = xcInv.scalarMult(X);
        Point Xhat = XhatEnc.add(XhatDec);
        Point XhatInv = Xhat.negate();

        byte[] keyMaterial = "".getBytes();
        for (Point C : Cs) {
            Point keyPointHat = C.add(XhatInv);
            byte[] keyPart = keyPointHat.pubLoad();
            int lastpos = keyMaterial.length;
            keyMaterial = Arrays.copyOfRange(keyMaterial, 0, keyMaterial.length + keyPart.length);
            System.arraycopy(keyPart, 0, keyMaterial, lastpos, keyPart.length);
        }

        return keyMaterial;
    }

    public String toString() {
        return String.format("Cs.length: %d\n" +
                "XhatEnc: %bytes\n" +
                "X: %bytes", Cs.size(), XhatEnc, X);
    }
}
