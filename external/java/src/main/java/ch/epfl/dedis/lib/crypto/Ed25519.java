package ch.epfl.dedis.lib.crypto;

import net.i2p.crypto.eddsa.math.Curve;
import net.i2p.crypto.eddsa.math.Field;
import net.i2p.crypto.eddsa.spec.EdDSANamedCurveSpec;
import net.i2p.crypto.eddsa.spec.EdDSANamedCurveTable;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.ByteBuffer;
import java.util.UUID;

/**
 * dedis/lib
 * Ed25519.java
 * Purpose: Getting the warm cozy feeling of having the power to add, subtract,
 * scalar multiply and do other fancy things with points and scalars.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */
public class Ed25519 {
    private final static Logger logger = LoggerFactory.getLogger(Ed25519.class);

    public static final int pubLen = 30;
    public static final EdDSANamedCurveSpec ed25519 = EdDSANamedCurveTable.getByName("Ed25519");
    public static Curve curve = ed25519.getCurve();
    public static Field field = curve.getField();
    public static Point base = new Point(Ed25519.ed25519.getB());
    public static Scalar prime_order = new Scalar("EDD3F55C1A631258D69CF7A2DEF9DE1400000000000000000000000000000010", false);

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

}
