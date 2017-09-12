import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class RosterTest {
    @Test
    void testRoster() {
        String serverFile = "[[servers]]\n" +
                "  Address = \"tcp://127.0.0.1:7002\"\n" +
                "  Public = \"SbcS4dAufwfeRZwRbhHl0mxDpuPgh2vfrfTncsWcvTI=\"\n" +
                "  Description = \"Conode_1\"\n" +
                "[[servers]]\n" +
                "  Address = \"tcp://127.0.0.1:7004\"\n" +
                "  Public = \"18MXk3cMx4PIGt0ww812o7JuGoxwfwJbTEbQug1gfhE=\"\n" +
                "  Description = \"Conode_2\"\n" +
                "[[servers]]\n" +
                "  Address = \"tcp://127.0.0.1:7006\"\n" +
                "  Public = \"rnZDQwJOQ0bJ9R3CYSLkrUBUmLHqd18LmU4VKJhJtNw=\"\n" +
                "  Description = \"Conode_3\"";
        Roster r = new Roster(serverFile);
        assertEquals(3, r.Nodes.size());
        assertEquals("Conode_2", r.Nodes.get(1).Description);
    }
}