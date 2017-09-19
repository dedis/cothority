package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Crypto;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.proto.RosterProto;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class RosterTest {
    private static Roster r = new Roster(LocalRosters.group);
    private static Crypto.Point agg = new Crypto.Point(LocalRosters.aggregate);

    @Test
    void testRoster() {
        assertEquals(3, r.Nodes.size());
        assertEquals("Conode_2", r.Nodes.get(1).Description);
    }

    @Test
    void testAggregate() {
        Crypto.Point pub = r.Nodes.get(0).Public.add(r.Nodes.get(1).Public);
        pub = pub.add(r.Nodes.get(2).Public);
        assertTrue(pub.equals(agg));
    }

    @Test
    void testProto() throws Exception {
        RosterProto.Roster r_proto = r.getProto();
        assertEquals(3, r_proto.getListList().size());
        assertArrayEquals(r_proto.getAggregate().toByteArray(), agg.toBytes());
        assertEquals(16, r_proto.getId().toByteArray().length);
    }
}