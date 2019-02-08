package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.PointFactory;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.proto.OnetProto;
import org.junit.jupiter.api.Test;

import java.net.URI;
import java.net.URL;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.Arrays;

import static ch.epfl.dedis.integration.TestServerController.buildURI;
import static org.junit.jupiter.api.Assertions.*;

class RosterTest {
    private static final Point SAMPLE_CONODE_PUB_1 = PointFactory.getInstance().fromToml("Ed25519", "8EDCBA0ACD1930F46D7293F6F6FDCE71A8DBFD7C5056C333DA2F20530AF930D5");
    private static final Point SAMPLE_CONODE_PUB_2 = PointFactory.getInstance().fromToml("Ed25519", "097318CB75DD5ED87983ACF47BFC4B064351107FE7F0FCCD2E28786168A3FCF2");
    private static final Point SAMPLE_CONODE_PUB_3 = PointFactory.getInstance().fromToml("Ed25519", "D5F268F8E66DA23CB92B407C55CC6474F183CCE54853F17D3CF27729E0BCB33A");
    private static final Point SAMPLE_CONODE_PUB_4 = PointFactory.getInstance().fromToml("Ed25519", "402552116B5056CC6B989BAE9A8DFD8BF0C1A2714FB820F0472C096AB5D148D8");

    private static final URI SAMPLE_CONODE_1 = buildURI("tls://testnode1:7772");
    private static final URI SAMPLE_CONODE_2 = buildURI("tls://testnode2:7774");
    private static final URI SAMPLE_CONODE_3 = buildURI("tls://testnode3:7776");
    private static final URI SAMPLE_CONODE_4 = buildURI("tls://testnode4:7778");

    private static final String SAMPLE_AGGREGATE = "465657D7E7C12F8CA355E5127AF2ABA43D4DC75A2B433FA63567D527C676F2A4";

    private static Roster r = new Roster(Arrays.asList(
            new ServerIdentity(SAMPLE_CONODE_1, SAMPLE_CONODE_PUB_1),
            new ServerIdentity(SAMPLE_CONODE_2, SAMPLE_CONODE_PUB_2),
            new ServerIdentity(SAMPLE_CONODE_3, SAMPLE_CONODE_PUB_3),
            new ServerIdentity(SAMPLE_CONODE_4, SAMPLE_CONODE_PUB_4)));

    private static Point agg = PointFactory.getInstance().fromToml("Ed25519", SAMPLE_AGGREGATE);

    @Test
    void testRoster() {
        assertEquals(4, r.getNodes().size());
    }

    @Test
    void testAggregate() {
        Point pub = r.getNodes().get(0).getPublic().add(r.getNodes().get(1).getPublic());
        pub = pub.add(r.getNodes().get(2).getPublic()).add(r.getNodes().get(3).getPublic());
        assertEquals(pub, agg);
    }

    @Test
    void testProto() {
        OnetProto.Roster r_proto = r.toProto();
        assertEquals(4, r_proto.getListList().size());
        assertArrayEquals(r_proto.getAggregate().toByteArray(), agg.toProto().toByteArray());
        assertEquals(16, r_proto.getId().toByteArray().length);
    }

    @Test
    void testFromToml() throws Exception {
        ClassLoader loader = getClass().getClassLoader();
        URL filepath = loader.getResource("public.toml");
        assertNotNull(filepath);

        String content = new String(Files.readAllBytes(Paths.get(filepath.toURI())));

        Roster roster = Roster.FromToml(content);
        assertEquals(7, roster.getNodes().size());
        assertTrue(roster.getNodes().get(0).getServiceIdentities().size() > 0);
    }
}

