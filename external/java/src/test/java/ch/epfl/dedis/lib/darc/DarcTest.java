package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

public class DarcTest {
    static Ed25519Signer owner;
    static Ed25519Signer user;
    static Darc darc;

    @BeforeAll
    static void initAll() throws CothorityCryptoException {
        owner = new Ed25519Signer();
        user = new Ed25519Signer();
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
        darc2.setEvolution(darc, null, owner);

        Darc darc3 = darc.copy();
        darc3.addUser(user);
        darc3.setEvolution(darc2, null, owner);

        assertTrue(darc2.verifyEvolution(darc));
        assertFalse(darc3.verifyEvolution(darc));
        assertTrue(darc3.verifyEvolution(darc2));
        assertTrue(darc.getBaseId().equals(darc2.getBaseId()));
        assertTrue(darc2.getBaseId().equals(darc3.getBaseId()));
    }

    @Test
    void proto() throws Exception {
        darc.incVersion();
        DarcProto.Darc proto = darc.toProto();
        Darc darc2 = new Darc(proto);
        assertTrue(darc.equals(darc2));
        assertTrue(darc.getVersion() == 1);
        assertTrue(darc2.getVersion() == 1);
    }

}
