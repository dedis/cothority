package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.proto.ServerIdentityProto;
import ch.epfl.dedis.proto.StatusProto;
import com.google.protobuf.ByteString;
import org.junit.jupiter.api.Test;

import javax.xml.bind.DatatypeConverter;

import static ch.epfl.dedis.LocalRosters.CONODE_PUB_1;
import static ch.epfl.dedis.LocalRosters.CONODE_1;
import static ch.epfl.dedis.LocalRosters.ids;
import static org.junit.jupiter.api.Assertions.*;

class ServerIdentityTest {
    static ServerIdentity si = new ServerIdentity(CONODE_1, CONODE_PUB_1);

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
        // TODO: there is not much value in this test
        assertEquals(CONODE_1.toString(), si.getAddress().toString());
        assertNotEquals(null, si.Public);
    }

    @Test
    void testProto(){
        ServerIdentityProto.ServerIdentity si_proto = si.getProto();
        byte[] id = DatatypeConverter.parseHexBinary(ids[0]);
        assertArrayEquals(ByteString.copyFrom(id).toByteArray(), si_proto.getId().toByteArray());
    }
}