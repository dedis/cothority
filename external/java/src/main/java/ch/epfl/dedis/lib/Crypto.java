package ch.epfl.dedis.lib;

import ch.epfl.dedis.ocs.Account;
import ch.epfl.dedis.proto.OCSProto;
import ch.epfl.dedis.proto.SkipBlockProto;
import com.google.protobuf.ByteString;
import net.i2p.crypto.eddsa.EdDSAPrivateKey;
import net.i2p.crypto.eddsa.EdDSAPublicKey;
import net.i2p.crypto.eddsa.math.Curve;
import net.i2p.crypto.eddsa.math.Field;
import net.i2p.crypto.eddsa.math.FieldElement;
import net.i2p.crypto.eddsa.math.GroupElement;
import net.i2p.crypto.eddsa.spec.EdDSANamedCurveSpec;
import net.i2p.crypto.eddsa.spec.EdDSANamedCurveTable;
import net.i2p.crypto.eddsa.spec.EdDSAPrivateKeySpec;
import net.i2p.crypto.eddsa.spec.EdDSAPublicKeySpec;

import javax.xml.bind.DatatypeConverter;
import java.nio.ByteBuffer;
import java.security.PublicKey;
import java.util.Arrays;
import java.util.Random;
import java.util.UUID;

/**
 * dedis/lib
 * Crypto.java
 * Purpose: Getting the warm cozy feeling of having the power to add, subtract,
 * scalar multiply and do other fancy things with points and scalars.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */
public class Crypto {
    public static final int pubLen = 30;
    public static final EdDSANamedCurveSpec ed25519 = EdDSANamedCurveTable.getByName("Ed25519");
    public static Curve curve = ed25519.getCurve();
    public static Field field = curve.getField();
    public static Scalar prime_order = new Scalar("EDD3F55C1A631258D69CF7A2DEF9DE1400000000000000000000000000000010");
    public static Point base = new Point(Crypto.ed25519.getB());
    // TODO: use CBC here and transmit the IV - or make sure that in this
    // special case ECB is secure.
    public static String algo = "AES/ECB/PKCS5Padding";
    public static String algoKey = "AES";


    public static byte[] uuid4() {
        ByteBuffer bb = ByteBuffer.wrap(new byte[16]);
        UUID uuid = UUID.randomUUID();
        bb.putLong(uuid.getMostSignificantBits());
        bb.putLong(uuid.getLeastSignificantBits());
        return bb.array();
    }

    public static byte[] reverse(byte[] little_endian) {
        byte[] big_endian = new byte[32];
        for (int i = 0; i < 32; i++) {
            big_endian[i] = little_endian[31 - i];
        }
        return big_endian;
    }

    public static class Point {
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
            element = new GroupElement(curve, b);
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
            EdDSAPublicKeySpec spec = new EdDSAPublicKeySpec(element, ed25519);
            return new EdDSAPublicKey(spec);
        }

        public Point negate() {
            return new Point(element.toP3().negate());
        }

        public byte[] pubLoad() throws CryptoException {
            byte[] bytes = toBytes();
            int len = bytes[0];
            if (len > pubLen || len < 0) {
                throw new CryptoException("doesn't seem to be a valid point");
            }
            return Arrays.copyOfRange(bytes, 1, len + 1);
        }


        public static Point pubStore(byte[] data) throws CryptoException {
            if (data.length > pubLen) {
                throw new CryptoException("too much data for point");
            }

            byte[] bytes = new byte[32];
            bytes[0] = (byte) data.length;
            System.arraycopy(data, 0, bytes, 1, data.length);
            for (bytes[31] = (byte) 0; bytes[31] < (byte) 127; bytes[31]++) {
                try {
                    Point e = new Point(bytes);
                    if (!e.scalarMult(prime_order).isZero()) {
                        continue;
                    }
                    ;
                    return e;
                } catch (IllegalArgumentException e) {
                    // Will fail in about 87.5%, so try again.
                }
            }
            throw new CryptoException("did not find matching point!?!");
        }
    }

    public static class Scalar {
        public FieldElement field;

        public Scalar(String str) {
            this(DatatypeConverter.parseHexBinary(str));
        }

        public Scalar(byte[] b) {
            this(Crypto.field.fromByteArray(b));
        }

        public Scalar(FieldElement f) {
            field = f;
        }

        public String toString() {
            return DatatypeConverter.printHexBinary(getLittleEndian());
        }

        public ByteString toProto() {
            return ByteString.copyFrom(reduce().getLittleEndian());
        }

        public byte[] toBytes(){
            return reduce().getLittleEndian();
        }

        public Scalar reduce() {
            return new Scalar(ed25519.getScalarOps().reduce(getLittleEndianFull()));
        }

        public Scalar copy() {
            return new Scalar(getLittleEndian());
        }

        public boolean equals(Scalar other) {
            return Arrays.equals(field.toByteArray(), other.field.toByteArray());
        }

        public Scalar addOne() {
            return new Scalar(field.addOne());
        }

        public byte[] getBigEndian() {
            return reverse(getLittleEndian());
        }

        public byte[] getLittleEndian() {
            return field.toByteArray();
        }

        public byte[] getLittleEndianFull() {
            return Arrays.copyOfRange(getLittleEndian(), 0, 64);
        }

        public Scalar add(Scalar b) {
            return new Scalar(field.add(b.field));
        }

        public Scalar sub(Scalar b) {
            return new Scalar(field.subtract(b.field));
        }

        public Scalar invert() {
            return new Scalar(field.invert());
        }

        public Scalar negate() {
            return prime_order.sub(this).reduce();
        }

        public boolean isZero() {
            return !reduce().field.isNonZero();
        }

        public Point mul(Point p) {
            if (p == null) {
                p = base;
            }
            return p.scalarMult(this);
        }

        public EdDSAPrivateKey getPrivate() {
            EdDSAPrivateKeySpec spec = new EdDSAPrivateKeySpec(ed25519, getLittleEndianFull());
            return new EdDSAPrivateKey(spec);
        }
    }

    public static class CryptoException extends Exception {
        public CryptoException(String m) {
            super(m);
        }
    }

    public static class KeyPair {
        public Scalar Scalar;
        public Point Point;

        public KeyPair() {
            byte[] seed = new byte[Crypto.field.getb() / 8];
            new Random().nextBytes(seed);
            Scalar = new Scalar(seed);
            Point = Scalar.mul(null);
        }
    }

    public static class SchnorrSig {
        public Scalar challenge;
        public Scalar response;

        public SchnorrSig(byte[] data, Account signer) {
            KeyPair kp = new KeyPair();
            // TODO: create correct signature
            challenge = kp.Scalar.reduce();
            response = challenge;
        }

        public SkipBlockProto.SchnorrSig toProto() {
            SkipBlockProto.SchnorrSig.Builder ss =
                    SkipBlockProto.SchnorrSig.newBuilder();
            ss.setChallenge(challenge.toProto());
            ss.setResponse(response.toProto());
            return ss.build();
        }
    }
}
