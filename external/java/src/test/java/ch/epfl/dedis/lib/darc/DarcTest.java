package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public class DarcTest {
    static Ed25519Signer owner;
    static Ed25519Signer user;
    static Darc darc;

    @BeforeAll
    static void initAll() throws Exception {
        owner = new Ed25519Signer();
        user = new Ed25519Signer();
        darc = new Darc(owner, null, null);
    }

    @Test
    void instantiateDarc() throws Exception{
        Darc darcCopy = new Darc(darc.ToProto());
        assertArrayEquals(darc.ID(), darcCopy.ID());

        Darc darc2 = darc.Copy();
        darc2.AddUser(user);
        assertNotEquals(darc2.ID(), darc.ID());
    }

    @Test
    void evolveDarc() throws Exception{
        Darc darc2 = darc.Copy();
        darc2.AddUser(user);
        darc2.SetEvolution(darc, null, owner);

        Darc darc3 = darc.Copy();
        darc3.AddUser(user);
        darc3.SetEvolution(darc2, null, owner);

        assertTrue(darc2.VerifyEvolution(darc));
        assertFalse(darc3.VerifyEvolution(darc));
        assertTrue(darc3.VerifyEvolution(darc2));
    }

    @Test
    void proto() throws Exception{
        darc.IncVersion();
        DarcProto.Darc proto = darc.ToProto();
        Darc darc2 = new Darc(proto);
        assertTrue(darc.equals(darc2));
        assertTrue(darc.GetVersion() == 1);
        assertTrue(darc2.GetVersion() == 1);
    }

}
