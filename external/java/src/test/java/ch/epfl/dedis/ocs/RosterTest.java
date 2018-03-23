package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.proto.RosterProto;
import org.junit.jupiter.api.Test;

import java.net.URI;
import java.util.Arrays;

import static ch.epfl.dedis.integration.TestServerController.buildURI;
import static org.junit.jupiter.api.Assertions.*;

class RosterTest {
    private static final String SAMPLE_CONODE_PUB_1 = "8EDCBA0ACD1930F46D7293F6F6FDCE71A8DBFD7C5056C333DA2F20530AF930D5";
    private static final String SAMPLE_CONODE_PUB_2 = "097318CB75DD5ED87983ACF47BFC4B064351107FE7F0FCCD2E28786168A3FCF2";
    private static final String SAMPLE_CONODE_PUB_3 = "D5F268F8E66DA23CB92B407C55CC6474F183CCE54853F17D3CF27729E0BCB33A";
    private static final String SAMPLE_CONODE_PUB_4 = "402552116B5056CC6B989BAE9A8DFD8BF0C1A2714FB820F0472C096AB5D148D8";

    private static final URI SAMPLE_CONODE_1 = buildURI("tcp://testnode1:7002");
    private static final URI SAMPLE_CONODE_2 = buildURI("tcp://testnode2:7004");
    private static final URI SAMPLE_CONODE_3 = buildURI("tcp://testnode3:7006");
    private static final URI SAMPLE_CONODE_4 = buildURI("tcp://testnode4:7008");

    private static final String SAMPLE_AGGREGATE = "465657D7E7C12F8CA355E5127AF2ABA43D4DC75A2B433FA63567D527C676F2A4";

    private static Roster r = new Roster(Arrays.asList(
            new ServerIdentity(SAMPLE_CONODE_1, SAMPLE_CONODE_PUB_1),
            new ServerIdentity(SAMPLE_CONODE_2, SAMPLE_CONODE_PUB_2),
            new ServerIdentity(SAMPLE_CONODE_3, SAMPLE_CONODE_PUB_3),
            new ServerIdentity(SAMPLE_CONODE_4, SAMPLE_CONODE_PUB_4)));

    //private static Roster r = ConnectingWithTomlConfig.constructRosterWithTomlConfig(LocalRosters.firstToml);

    private static Point agg = new Point(SAMPLE_AGGREGATE);

    @Test
    void testRoster() {
        assertEquals(4, r.getNodes().size());
    }

    @Test
    void testAggregate() {
        Point pub = r.getNodes().get(0).Public.add(r.getNodes().get(1).Public);
        pub = pub.add(r.getNodes().get(2).Public).add(r.getNodes().get(3).Public);
        assertTrue(pub.equals(agg));
    }

    @Test
    void testProto() throws Exception {
        RosterProto.Roster r_proto = r.getProto();
        assertEquals(4, r_proto.getListList().size());
        assertArrayEquals(r_proto.getAggregate().toByteArray(), agg.toBytes());
        assertEquals(16, r_proto.getId().toByteArray().length);
    }
}

