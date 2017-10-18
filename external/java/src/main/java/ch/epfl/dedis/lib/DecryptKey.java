package ch.epfl.dedis.lib;

import ch.epfl.dedis.ocs.Account;
import ch.epfl.dedis.proto.OCSProto;

import javax.crypto.Cipher;
import javax.crypto.spec.SecretKeySpec;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * dedis/lib
 * DecryptKey.java
 * Purpose: Does the onchain-secrets algorithm to retrieve the symmetric from the cothority.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */

public class DecryptKey {
    public List<Crypto.Point> Cs;
    public Crypto.Point XhatEnc;
    public Crypto.Point X;

    public DecryptKey() {
        Cs = new ArrayList<>();
    }

    public DecryptKey(OCSProto.DecryptKeyReply reply, Crypto.Point X) {
        this();
        reply.getCsList().forEach(C -> Cs.add(new Crypto.Point(C)));
        XhatEnc = new Crypto.Point(reply.getXhatEnc());
        this.X = X;
    }

    public byte[] getKeyMaterial(OCSProto.OCSWrite write, Account reader) throws Exception {
        List<Crypto.Point> Cs = new ArrayList<>();
        write.getCsList().forEach(cs -> Cs.add(new Crypto.Point(cs)));

        // Use our private key to decrypt the re-encryption key and use it
        // to recover the symmetric key.
        Crypto.Scalar xc = reader.Scalar.reduce();
        Crypto.Scalar xcInv = xc.negate();
        Crypto.Point XhatDec = xcInv.mul(X);
        Crypto.Point Xhat = XhatEnc.add(XhatDec);
        Crypto.Point XhatInv = Xhat.negate();

        byte[] keyMaterial = "".getBytes();
        for (Crypto.Point C : Cs) {
            Crypto.Point keyPointHat = C.add(XhatInv);
            try {
                byte[] keyPart = keyPointHat.pubLoad();
                int lastpos = keyMaterial.length;
                keyMaterial = Arrays.copyOfRange(keyMaterial, 0, keyMaterial.length + keyPart.length);
                System.arraycopy(keyPart, 0, keyMaterial, lastpos, keyPart.length);
            } catch (Crypto.CryptoException c) {
                c.printStackTrace();
                System.out.println("couldn't extract data! " + c.toString());
            }
        }

        return keyMaterial;
    }

    public String toString() {
        return String.format("Cs.length: %d\n" +
                "XhatEnc: %bytes\n" +
                "X: %bytes", Cs.size(), XhatEnc, X);
    }
}
