import org.junit.jupiter.api.Test;
import proto.StatusProto;

import static org.junit.jupiter.api.Assertions.*;

class ServerIdentityTest {
    String tomlFile = "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7002\"\n" +
            "  Public = \"5eI+WFOaCdMhHY+g+zR11IZV4MBtg+k8jm59FqqHwQY=\"\n" +
            "  Description = \"Conode_1\"";

    @Test
    void testGetStatus() {
        ServerIdentity s = new ServerIdentity(tomlFile);
        try {
            StatusProto.Response resp = s.GetStatus();
            assertNotNull(resp);
//            System.out.println(resp.toString());
        } catch (Exception e) {
            System.out.println(e.toString());
            assertFalse(true);
        }
    }

    @Test
    void testCreate() {
        ServerIdentity s = new ServerIdentity(tomlFile);
        assertEquals("tcp://127.0.0.1:7002", s.Address);
        assertEquals("127.0.0.1:7003", s.AddressWebSocket());
        assertEquals("Conode_1", s.Description);
        assertNotEquals(null, s.Public);
    }
}