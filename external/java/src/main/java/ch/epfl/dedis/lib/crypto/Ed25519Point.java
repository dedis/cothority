package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;
import net.i2p.crypto.eddsa.EdDSAPublicKey;
import net.i2p.crypto.eddsa.math.GroupElement;
import net.i2p.crypto.eddsa.spec.EdDSAPublicKeySpec;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.xml.bind.DatatypeConverter;
import java.security.PublicKey;
import java.util.Arrays;

public class Ed25519Point implements Point {
    private final static Logger logger = LoggerFactory.getLogger(Ed25519Point.class);

    private GroupElement element;

    public Ed25519Point(ByteString pub) {
        this(pub.toByteArray());
    }

    public Ed25519Point(PublicKey pub) {
        this(Arrays.copyOfRange(pub.getEncoded(), 12, 44));
    }

    public Ed25519Point(String str) {
        this(DatatypeConverter.parseHexBinary(str));
    }

    public Ed25519Point(byte[] b) {
        element = new GroupElement(Ed25519.curve, b);
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

    public boolean equals(Point other) {
        return Arrays.equals(element.toByteArray(), convert(other).element.toByteArray());
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
        return ByteString.copyFrom(toBytes());
    }

    public byte[] toBytes() {
        return element.toByteArray();
    }

    public boolean isZero() {
        return !element.toP2().getY().isNonZero();
    }

    public String toString() {
        return DatatypeConverter.printHexBinary(toBytes());
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
            logger.info(DatatypeConverter.printHexBinary(bytes));
            throw new CothorityCryptoException("doesn't seem to be a valid point");
        }
        return Arrays.copyOfRange(bytes, 1, len + 1);
    }


    public static Point embed(byte[] data) throws CothorityCryptoException {
        if (data.length > Ed25519.pubLen) {
            throw new CothorityCryptoException("too much data for point");
        }

        byte[] bytes = new byte[32];
        bytes[0] = (byte) data.length;
        System.arraycopy(data, 0, bytes, 1, data.length);
        for (bytes[31] = (byte) 0; bytes[31] < (byte) 127; bytes[31]++) {
            try {
                Ed25519Point e = new Ed25519Point(bytes);
                if (!e.mul(Ed25519.prime_order).isZero()) {
                    continue;
                }
                return e;
            } catch (IllegalArgumentException e) {
                // Will fail in about 87.5%, so try again.
            }
        }
        throw new CothorityCryptoException("did not find matching point!?!");
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
