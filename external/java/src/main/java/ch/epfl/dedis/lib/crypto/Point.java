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

public class Point {
    private final static Logger logger = LoggerFactory.getLogger(Point.class);

    public GroupElement element;

    public Point(ByteString pub) {
        this(pub.toByteArray());
    }

    public Point(PublicKey pub) {
        this(Arrays.copyOfRange(pub.getEncoded(), 12, 44));
    }

    public Point(String str) {
        this(DatatypeConverter.parseHexBinary(str));
    }

    public Point(byte[] b) {
        element = new GroupElement(Ed25519.curve, b);
    }

    public Point(Point p) {
        this(p.element);
    }

    public Point(GroupElement e) {
        element = e;
    }

    public Point copy() {
        return new Point(this);
    }

    public boolean equals(Point other) {
        return Arrays.equals(element.toByteArray(), other.element.toByteArray());
    }

    public Point scalarMult(Scalar s) {
        element = element.toP3();
        element.precompute(true);
        return new Point(element.scalarMultiply(s.getLittleEndian()));
    }

    public Point add(Point other) {
        return new Point(element.toP3().add(other.element.toCached()));
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
        return new Point(element.toP3().negate());
    }

    public byte[] pubLoad() throws CothorityCryptoException {
        byte[] bytes = toBytes();
        int len = bytes[0];
        if (len > Ed25519.pubLen || len < 0) {
            logger.info(DatatypeConverter.printHexBinary(bytes));
            throw new CothorityCryptoException("doesn't seem to be a valid point");
        }
        return Arrays.copyOfRange(bytes, 1, len + 1);
    }


    public static Point pubStore(byte[] data) throws CothorityCryptoException {
        if (data.length > Ed25519.pubLen) {
            throw new CothorityCryptoException("too much data for point");
        }

        byte[] bytes = new byte[32];
        bytes[0] = (byte) data.length;
        System.arraycopy(data, 0, bytes, 1, data.length);
        for (bytes[31] = (byte) 0; bytes[31] < (byte) 127; bytes[31]++) {
            try {
                Point e = new Point(bytes);
                if (!e.scalarMult(Ed25519.prime_order).isZero()) {
                    continue;
                }
                return e;
            } catch (IllegalArgumentException e) {
                // Will fail in about 87.5%, so try again.
            }
        }
        throw new CothorityCryptoException("did not find matching point!?!");
    }
}
