package ch.epfl.dedis.lib.crypto;

import com.google.protobuf.ByteString;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * It tests different creation of points from different inputs and kinds of points
 */
class PointFactoryTest {
    static private final String ED25519_POINT = "3B6A27BCCEB6A42D62A3A8D02A6F0D73653215771DE243A63AC048A18B59DA29";
    static private final String BN256_POINT = "01021817b72430e4ff2178726594c4b3941ab43d055af0fdad605b857fb98849e31" +
            "e7465981004a9f955397a920bbae310912f2fff7d5df5e11a5a9df325b1acff1f957a894d5f94513830f44d01273bc4f0ac67" +
            "318902ba3722fd799627d8c0ad59bb378cb9ea08e1176ece8860f29d057208ade76fbc88d6d22350772e3776fb";

    @Test
    void testFromProto() throws Exception {
        Point p1 = new Ed25519Point(ED25519_POINT);
        Point p2 = new Bn256G2Point(BN256_POINT);

        assertEquals(p1, PointFactory.getInstance().fromProto(p1.toProto()));
        assertEquals(p2, PointFactory.getInstance().fromProto(p2.toProto()));
        assertNull(PointFactory.getInstance().fromProto(ByteString.EMPTY));
        assertNull(PointFactory.getInstance().fromProto(p1.toProto().substring(8)));
    }

    @Test
    void testFromToml() throws Exception {
        Point p1 = new Ed25519Point(ED25519_POINT);
        Point p2 = new Bn256G2Point(BN256_POINT);

        assertEquals(p1, PointFactory.getInstance().fromToml(PointFactory.SUITE_ED25519, ED25519_POINT));
        assertEquals(p2, PointFactory.getInstance().fromToml(PointFactory.SUITE_BN256, BN256_POINT));
        assertNull(PointFactory.getInstance().fromToml("", ED25519_POINT));
        assertNull(PointFactory.getInstance().fromToml(PointFactory.SUITE_ED25519, ""));
    }
}
