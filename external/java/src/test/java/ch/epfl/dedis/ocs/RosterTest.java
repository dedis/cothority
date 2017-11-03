package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.proto.RosterProto;
import org.junit.jupiter.api.Test;

import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.*;

class RosterTest {
    private static Roster r = new Roster(Arrays.asList(
            new ServerIdentity(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1),
            new ServerIdentity(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2),
            new ServerIdentity(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3)));

    //private static Roster r = ConnectingWithTomlConfig.constructRosterWithTomlConfig(LocalRosters.firstToml);

    private static Point agg = new Point(LocalRosters.aggregate);

    @Test
    void testRoster() {
        assertEquals(3, r.getNodes().size());
    }

    @Test
    void testAggregate() {
        Point pub = r.getNodes().get(0).Public.add(r.getNodes().get(1).Public);
        pub = pub.add(r.getNodes().get(2).Public);
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