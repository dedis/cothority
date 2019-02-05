package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;
import net.i2p.crypto.eddsa.EdDSAPublicKey;
import net.i2p.crypto.eddsa.math.GroupElement;
import net.i2p.crypto.eddsa.spec.EdDSAPublicKeySpec;
import org.bouncycastle.crypto.Xof;
import org.bouncycastle.crypto.digests.SHAKEDigest;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.security.SecureRandom;
import java.util.Arrays;
import java.util.Random;

public class Ed25519Point implements Point {
    final static byte[] marshalID = "ed.point".getBytes();

    private final static Logger logger = LoggerFactory.getLogger(Ed25519Point.class);

    private GroupElement element;

    public Ed25519Point(byte[] b) throws CothorityCryptoException {
        if (b.length != 40 && b.length != 32) {
            throw new CothorityCryptoException("Wrong Edward25519 format");
        }

        if (Arrays.equals(marshalID, Arrays.copyOfRange(b, 0, 8))) {
            b = Arrays.copyOfRange(b, 8, b.length);
        }
        element = new GroupElement(Ed25519.curve, b);
    }

    public Ed25519Point(String str) throws CothorityCryptoException {
        this(Hex.parseHexBinary(str));
    }

    public Ed25519Point(Point p) {
        this(convert(p).element);
    }

    public Ed25519Point(GroupElement e) {
        element = e;
    }

    public Point copy() {
        return new Ed25519Point(this);
    }

    @Override
    public boolean equals(Object other) {
        if (!(other instanceof Ed25519Point)) {
            return false;
        }

        return Arrays.equals(element.toByteArray(), convert((Ed25519Point) other).element.toByteArray());
    }

    public Point mul(Scalar s) {
        element = element.toP3();
        element.precompute(true);
        return new Ed25519Point(element.scalarMultiply(s.getLittleEndian()));
    }

    public Point add(Point other) {
        Ed25519Point p = convert(other);
        return new Ed25519Point(element.toP3().add(p.element.toCached()));
    }

    public ByteString toProto() {
        return ByteString.copyFrom(marshalID).concat(ByteString.copyFrom(toBytes()));
    }

    public byte[] toBytes() {
        return element.toByteArray();
    }

    public boolean isZero() {
        return this.element.equals(Ed25519.curve.getZero(GroupElement.Representation.P3));
    }

    public Point getZero() {
        return new Ed25519Point(Ed25519.curve.getZero(GroupElement.Representation.P3));
    }

    public String toString() {
        return Hex.printHexBinary(toBytes());
    }

    public EdDSAPublicKey toEdDSAPub() {
        EdDSAPublicKeySpec spec = new EdDSAPublicKeySpec(element, Ed25519.ed25519);
        return new EdDSAPublicKey(spec);
    }

    public Point negate() {
        return new Ed25519Point(element.toP3().negate());
    }

    public byte[] data() throws CothorityCryptoException {
        byte[] bytes = toBytes();
        int len = bytes[0];
        if (len > Ed25519.pubLen || len < 0) {
            logger.info(Hex.printHexBinary(bytes));
            throw new CothorityCryptoException("doesn't seem to be a valid point");
        }
        return Arrays.copyOfRange(bytes, 1, len + 1);
    }

    /**
     * Embed data into a point. If the data is longer than 29 bytes then the extra bytes are ignored.
     *
     * @param data is the data to embed.
     * @return the embedded point.
     */
    public static Point embed(byte[] data) {
        return embed(data, new SecureRandom());
    }

    /**
     * Embed data into a point. If the data is longer than 29 bytes then the extra bytes are ignored.
     *
     * @param data is the data to embed.
     * @param rand is the source of randomness to generate missing bytes in the point
     * @return the embedded point.
     */
    public static Point embed(byte[] data, Random rand) {
        SHAKEDigest d = new SHAKEDigest(256);
        // 128 is picked as the size of the seed because the Reseed function in the Go implementation is also 128
        byte[] buf = new byte[128];

        rand.nextBytes(buf);
        d.update(buf, 0, 128);

        return embed(data, d);
    }

    /**
     * Embed data into a point. If the data is longer than 29 bytes then the extra bytes are ignored.
     *
     * @param data is the data to embed
     * @param xof is the extendable output function used to fill the missing bytes in the point.
     * @return the embedded point.
     */
    public static Point embed(byte[] data, Xof xof) {
        int dataLen = 0;
        if (data != null) {
            dataLen = data.length;
        }
        int embedLen = (255 - 8 - 8) / 8;
        if (embedLen > dataLen) {
            embedLen = dataLen;
        }
        for (;;) {
            byte[] bytes = new byte[32];
            xof.doOutput(bytes, 0, 32);
            if (dataLen > 0) {
                bytes[0] = (byte) embedLen;
                System.arraycopy(data, 0, bytes, 1, embedLen);
            }

            Point P;
            try {
                P = new Ed25519Point(bytes);
            } catch (IllegalArgumentException e) {
                // not a valid point, try again
                continue;
            } catch (CothorityCryptoException e) {
                // this exception only throws if the byte array representation of the point has a wrong length,
                // but we set it statically so it cannot happen
                throw new RuntimeException(e.getMessage());
            }

            if (dataLen == 0) {
                P = P.mul(Ed25519.cofactor);
                if (P.isZero()) {
                    continue;
                }
                return P;
            }

            Point Q = P.copy();
            Q = Q.mul(Ed25519.prime_order);
            if (Q.isZero()) {
                return P;
            }
        }
    }

    public static Point base() {
        return Ed25519.base;
    }

    private static Ed25519Point convert(Point p) {
        if (!(p instanceof Ed25519Point)) {
            throw new IllegalArgumentException(String.format("Error thrown because you are trying to operate an Ed25519Point with a Point implementing class %s", p.getClass().getName()));
        }
        return (Ed25519Point) p;
    }

}
