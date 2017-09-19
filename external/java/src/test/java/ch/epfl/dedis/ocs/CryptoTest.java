package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Crypto;
import org.junit.jupiter.api.Test;

import javax.xml.bind.DatatypeConverter;

import static org.junit.jupiter.api.Assertions.*;

class CryptoTest {
    @Test
    void point() {
        String point = "3B6A27BCCEB6A42D62A3A8D02A6F0D73653215771DE243A63AC048A18B59DA29";
        Crypto.Point point2 = new Crypto.Point(point);
        assertEquals(point, point2.toString());
        assertEquals(32, point2.toBytes().length);
        assertTrue(new Crypto.Point(point2.toBytes()).equals(point2));

        byte[] point_bytes = DatatypeConverter.parseHexBinary("3B6A27BCCEB6A42D62A3A8D02A6F0D73653215771DE243A63AC048A18B59DA29");
        assertTrue(new Crypto.Point(point_bytes).equals(point2));
    }

    @Test
    void pubStoreLoad() throws Exception {
        String short_str = "short2";
        String long_str = "this is a string too long to be embedded";

        Crypto.Point pub = Crypto.Point.pubStore(short_str.getBytes());
        byte[] ret = pub.pubLoad();
        assertEquals(short_str, new String(ret));

        try {
            Crypto.Point.pubStore(long_str.getBytes());
            throw new Crypto.CryptoException("this should not pass");
        } catch (Crypto.CryptoException e) {
        }
    }

    @Test
    void toPrivate() {
        Crypto.KeyPair kp = new Crypto.KeyPair();

        Crypto.Point pub = kp.Scalar.mul(null);
        assertTrue(pub.equals(kp.Point));

        Crypto.Scalar onep = kp.Scalar.addOne();
        assertEquals(onep.getLittleEndian()[0], kp.Scalar.getLittleEndian()[0] + 1);
    }

    @Test
    void scalarMultiply() {
        String priv_str = "66F1874A926079F5907A26B57079B5583E42C3D0FDBB2B7B8638A8DBC1AD4622";
        String priv_str_reduced = "8C499C905D9A5445E440376FB385F72E3E42C3D0FDBB2B7B8638A8DBC1AD4602";
        String pub_str = "6ECFEB30C65BA92D16521DB20BA21C64F86E4CE294A733C66B38B691311078E6";
        Crypto.Scalar priv = new Crypto.Scalar(priv_str);
        Crypto.Point pub = priv.mul(null);
        assertEquals(pub_str, pub.toString());

        Crypto.Scalar priv_reduced = new Crypto.Scalar(priv_str_reduced);
        assertEquals(pub_str, priv_reduced.reduce().mul(null).toString());
    }

    @Test
    void endianness() {
        String priv_str = "66F1874A926079F5907A26B57079B5583E42C3D0FDBB2B7B8638A8DBC1AD4622";
        String priv_reduced_str = "8C499C905D9A5445E440376FB385F72E3E42C3D0FDBB2B7B8638A8DBC1AD4602";
        String priv1_reduced_str = "8D499C905D9A5445E440376FB385F72E3E42C3D0FDBB2B7B8638A8DBC1AD4602";
        String pub_str = "6ECFEB30C65BA92D16521DB20BA21C64F86E4CE294A733C66B38B691311078E6";
        String pub1_str = "FBDAFDA7941D5088990B8DAEAE35B2D7F3E3342B427ABFCF94664374A93C0719";

        Crypto.Scalar priv = new Crypto.Scalar(priv_str);
        assertEquals(priv_str, priv.toString());
        assertEquals(priv_reduced_str, priv.reduce().toString());

        Crypto.Point pub = priv.mul(null);
        assertEquals(pub_str, pub.toString());

        Crypto.Scalar priv_next = priv.reduce().addOne();
        assertEquals(priv1_reduced_str, priv_next.toString());
        assertEquals(pub1_str, priv_next.mul(null).toString());
    }

    @Test
    void reduce() {
        String priv_str = "66F1874A926079F5907A26B57079B5583E42C3D0FDBB2B7B8638A8DBC1AD4622";
        String priv_reduced_str = "8C499C905D9A5445E440376FB385F72E3E42C3D0FDBB2B7B8638A8DBC1AD4602";
        Crypto.Scalar priv = new Crypto.Scalar(priv_str);
        assertEquals(priv_str, priv.toString());
        assertEquals(priv_reduced_str, priv.reduce().toString());

        Crypto.Scalar reduced = new Crypto.Scalar(priv_reduced_str);
        assertEquals(priv_reduced_str, reduced.toString());

        Crypto.Scalar reduced2 = reduced.reduce();
        assertTrue(reduced2.equals(reduced));
        assertTrue(reduced2.reduce().equals(reduced));
        assertTrue(reduced2.reduce().reduce().equals(reduced));
    }

    @Test
    void negate(){
        Crypto.Scalar e = new Crypto.Scalar("762755eb09f5a1b3927d89625a90ac93351eba404aa0d0a62315985cc94ba304").reduce();
        Crypto.Scalar neg = e.negate();
        Crypto.Scalar sum = e.add(neg);
        assertTrue(sum.isZero());

        Crypto.Scalar f = new Crypto.Scalar("77aca071106e70a4431f6e4084693281cae145bfb55f2f59dcea67a336b45c0b");
        assertArrayEquals(neg.reduce().getLittleEndian(), f.getLittleEndian());
    }
}