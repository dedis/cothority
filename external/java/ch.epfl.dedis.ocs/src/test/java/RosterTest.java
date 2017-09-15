import net.i2p.crypto.eddsa.math.GroupElement;
import org.junit.jupiter.api.Test;
import proto.RosterProto;

import java.security.PublicKey;

import static org.junit.jupiter.api.Assertions.*;

class RosterTest {
    private static Roster r = new Roster(LocalRosters.group);
    private static PublicKey agg = Crypto.hexToPublic(LocalRosters.aggregate);

    @Test
    void testRoster() {
        assertEquals(3, r.Nodes.size());
        assertEquals("Conode_2", r.Nodes.get(1).Description);
    }

    @Test
    void testAggregate() {
        GroupElement pub = Crypto.add(r.Nodes.get(0).Public, r.Nodes.get(1).Public);
        pub = Crypto.add(pub, r.Nodes.get(2).Public);
        assertArrayEquals(pub.toByteArray(), Crypto.toBytes(agg));
    }

    @Test
    void testProto() throws Exception {
        RosterProto.Roster r_proto = r.getProto();
        assertEquals(3, r_proto.getListList().size());
        assertArrayEquals(r_proto.getAggregate().toByteArray(), Crypto.toBytes(agg));
        assertEquals(16, r_proto.getId().toByteArray().length);
    }
}