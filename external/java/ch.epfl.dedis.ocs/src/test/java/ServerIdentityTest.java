import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class ServerIdentityTest {
    @Test
    void testCreate() {
        String tomlFile = "[[servers]]\n" +
                "  Address = \"tcp://78.46.227.60:7770\"\n" +
                "  Public = \"5eI+WFOaCdMhHY+g+zR11IZV4MBtg+k8jm59FqqHwQY=\"\n" +
                "  Description = \"Ineiti's Cothority-server\"";
        ServerIdentity s = new ServerIdentity(tomlFile);
        assertEquals("tcp://78.46.227.60:7770", s.Address);
        assertEquals("Ineiti's Cothority-server", s.Description);
        assertNotEquals(null, s.Public);
    }
}