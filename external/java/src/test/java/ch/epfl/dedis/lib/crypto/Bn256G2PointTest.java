package ch.epfl.dedis.lib.crypto;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertNotNull;

class Bn256G2PointTest {
    @Test
    void testConstructors() {
        Point p = new Bn256G2Point("");
        assertNotNull(p);

        p = new Bn256G2Point(new byte[0]);
        assertNotNull(p); // TODO: error handling for bad points
    }
}
