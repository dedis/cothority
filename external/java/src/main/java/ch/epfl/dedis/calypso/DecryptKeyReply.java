package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.PointFactory;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.Calypso;

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
     * @param proto the input protobuf
     */
    public DecryptKeyReply(Calypso.DecryptKeyReply proto) {
        this.Cs = proto.getCsList().stream()
                .map(cs -> PointFactory.getInstance().fromProto(cs))
                .collect(Collectors.toList());
        this.XhatEnc = PointFactory.getInstance().fromProto(proto.getXhatenc());
        this.X = PointFactory.getInstance().fromProto(proto.getX());
    }

    /**
     * Recover the key material by decrypting each slice and merging all the slices. This has to be done because we use
     * ElGamal encryption that can only encrypt 30 bytes when using Ed25519.
     *
     * @param reader The secret key of the reader.
     * @return the key material
     * @throws CothorityCryptoException if something went wrong with decoding the point
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
