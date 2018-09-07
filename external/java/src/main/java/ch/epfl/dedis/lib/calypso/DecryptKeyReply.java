package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.Calypso;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

/**
 * The response of the DecryptKey RPC call.
 */
public class DecryptKeyReply {
    private List<Point> Cs;
    private Point XhatEnc;
    private Point X;

    /**
     * Construct from the protobuf representation.
     */
    public DecryptKeyReply(Calypso.DecryptKeyReply proto) {
        this.Cs = proto.getCsList().stream().map(Ed25519Point::new).collect(Collectors.toList());
        this.XhatEnc = new Ed25519Point(proto.getXhatenc());
        this.X = new Ed25519Point(proto.getX());
    }

    /**
     * Recover the key material by decrypting each slice and merging all the slices. This has to be done because we use
     * ElGamal encryption that can only encrypt 30 bytes when using Ed25519.
     *
     * @param reader The secret key of the reader.
     */
    public byte[] getKeyMaterial(Scalar reader) throws CothorityCryptoException {
        // Use our private key to decrypt the re-encryption key and use it
        // to recover the symmetric key.
        Scalar xc = reader.reduce();
        Scalar xcInv = xc.negate();
        Point XhatDec = X.mul(xcInv);
        Point Xhat = XhatEnc.add(XhatDec);
        Point XhatInv = Xhat.negate();

        byte[] keyMaterial = "".getBytes();
        for (Point C : this.Cs) {
            Point keyPointHat = C.add(XhatInv);
            byte[] keyPart = keyPointHat.data();
            int lastpos = keyMaterial.length;
            keyMaterial = Arrays.copyOfRange(keyMaterial, 0, keyMaterial.length + keyPart.length);
            System.arraycopy(keyPart, 0, keyMaterial, lastpos, keyPart.length);
        }

        return keyMaterial;
    }
}
