package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.proto.ServerIdentityProto;
import ch.epfl.dedis.proto.StatusProto;
import com.google.protobuf.ByteString;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import javax.xml.bind.DatatypeConverter;

import static org.junit.jupiter.api.Assertions.*;

class ServerIdentityTest {
    static ServerIdentity si = new ServerIdentity(LocalRosters.first);

    @BeforeAll
    static void initAll() throws Exception {
    }

    @Test
    void testGetStatus() {
        try {
            StatusProto.Response resp = si.GetStatus();
            assertNotNull(resp);
        } catch (Exception e) {
            System.out.println(e.toString());
            assertFalse(true);
        }
    }

    @Test
    void testCreate() {
        assertEquals("tcp://127.0.0.1:7002", si.Address);
        assertEquals("127.0.0.1:7003", si.AddressWebSocket());
        assertEquals("Conode_1", si.Description);
        assertNotEquals(null, si.Public);
    }

    @Test
    void testProto(){
        ServerIdentityProto.ServerIdentity si_proto = si.getProto();
        byte[] id = DatatypeConverter.parseHexBinary(LocalRosters.ids[0]);
        assertArrayEquals(ByteString.copyFrom(id).toByteArray(), si_proto.getId().toByteArray());
    }
}