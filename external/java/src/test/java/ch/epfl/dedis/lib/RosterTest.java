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
    void testRosterID() throws Exception {
        // Reference test with the go implementation
        String roBytes = "0a10be07d7e9bb4454879b1b61696e390b48129a010a2865642e706f696e749a93c8ddc9b3c7750b2c1b5ff2636aa" +
                "455dd10dca7d2f1e9f26674080ed68d1512400a0b53657276696365546573741207456432353531391a2865642e706f696e74b" +
                "fd2a1a547d750e87c14d6f1c11eedcc0628c43c8ff288421274085a00a501f91a10967578c5e8cc5a81af09eb18466940de221" +
                "66c6f63616c3a2f2f3132372e302e302e313a323030302a003a00129a010a2865642e706f696e74c9fcb5d21be1721b6c8c32d" +
                "f86e89812178556c9e3dbc211d27f3a602a4548fa12400a0b53657276696365546573741207456432353531391a2865642e706" +
                "f696e74a0647d4d217f67ced6084649f4d61a2a7786e6c55da648386b3e0802e97ac7a71a10ce118cfbd568559faa8660f99d4" +
                "ad3eb22166c6f63616c3a2f2f3132372e302e302e313a323030312a003a001a2865642e706f696e74553501567de11b0befd52" +
                "5c4485894c13f2f930958abcfa4bfd8d959951ab217";

        String id = "be07d7e9-bb44-5487-9b1b-61696e390b48";
        Roster ro = new Roster(OnetProto.Roster.parseFrom(Hex.parseHexBinary(roBytes)));

        assertEquals(id, ro.getID().toString());
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
        assertArrayEquals(UUIDType5.toBytes(r.getID()), r_proto.getId().toByteArray());
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

