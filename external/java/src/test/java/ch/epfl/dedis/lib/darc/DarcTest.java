package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcOCSProto;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.*;

public class DarcTest {
    static SignerEd25519 owner;
    static SignerEd25519 user;
    static Darc darc;

    @BeforeAll
    static void initAll() throws CothorityCryptoException {
        owner = new SignerEd25519();
        user = new SignerEd25519();
        darc = new Darc(owner, null, null);
    }

    @Test
    void instantiateDarc() throws Exception {
        Darc darcCopy = new Darc(darc.toProto());
        assertTrue(darc.getId().equals(darcCopy.getId()));

        Darc darc2 = darc.copy();
        darc2.addUser(user);
        assertFalse(darc2.getId().equals(darc.getId()));
    }

    @Test
    void differntiateDarcs() throws CothorityCryptoException {
        Darc darc1 = new Darc(owner, null, null);
        Darc darc2 = new Darc(owner, null, null);
        assertFalse(darc1.equals(darc2));
    }

    @Test
    void evolveDarc() throws Exception {
        Darc darc2 = darc.copy();
        darc2.addUser(user);
        darc2.setEvolution(darc, owner);

        Darc darc3 = darc.copy();
        darc3.addUser(user);
        darc3.setEvolution(darc2, owner);

        assertThrows(CothorityCryptoException.class, ()->darc2.verifyEvolution(darc));
        assertThrows(CothorityCryptoException.class, ()->darc3.verifyEvolution(darc));
        assertTrue(darc.getBaseId().equals(darc2.getBaseId()));
        assertTrue(darc2.getBaseId().equals(darc3.getBaseId()));
    }

    @Test
    void evolveDarcOffline() throws Exception {
        Darc darc2 = darc.copy();
        darc2.addUser(user);
        darc2.setEvolutionOffline(darc, null, owner);

        Darc darc3 = darc.copy();
        darc3.addUser(user);
        darc3.setEvolutionOffline(darc2, null, owner);

        assertTrue(darc2.verifyEvolution(darc));
        assertFalse(darc3.verifyEvolution(darc));
        assertTrue(darc3.verifyEvolution(darc2));
        assertTrue(darc.getBaseId().equals(darc2.getBaseId()));
        assertTrue(darc2.getBaseId().equals(darc3.getBaseId()));
    }

    @Test
    void proto() throws Exception {
        darc.incVersion();
        DarcOCSProto.Darc proto = darc.toProto();
        Darc darc2 = new Darc(proto);
        assertTrue(darc.equals(darc2));
        assertTrue(darc.getVersion() == 1);
        assertTrue(darc2.getVersion() == 1);
    }

    @Test
    void removeFromDarc() throws CothorityCryptoException{
        SignerEd25519 owner2 = new SignerEd25519();
        SignerEd25519 owner3 = new SignerEd25519();
        SignerEd25519 user2 = new SignerEd25519();
        SignerEd25519 user3 = new SignerEd25519();

        Darc darc = new Darc(owner, Arrays.asList(user, user2), null);
        darc.addOwner(owner2);
        assertEquals(2, darc.getOwners().size());
        assertEquals(2, darc.getUsers().size());

        darc.removeOwner(owner3.getIdentity());
        assertEquals(2, darc.getOwners().size());
        assertEquals(2, darc.getUsers().size());

        darc.removeOwner(owner2.getIdentity());
        assertEquals(1, darc.getOwners().size());
        assertEquals(2, darc.getUsers().size());

        darc.removeUser(user3.getIdentity());
        assertEquals(1, darc.getOwners().size());
        assertEquals(2, darc.getUsers().size());

        darc.removeUser(user2.getIdentity());
        assertEquals(1, darc.getOwners().size());
        assertEquals(1, darc.getUsers().size());
    }

}
