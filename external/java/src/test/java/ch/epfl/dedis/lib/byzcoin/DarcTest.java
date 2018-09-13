package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.byzcoin.darc.Darc;
import ch.epfl.dedis.lib.byzcoin.darc.SignerEd25519;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.*;


public class DarcTest {

    static SignerEd25519 owner;
    static SignerEd25519 user;
    static Darc darc;

    private final static Logger logger = LoggerFactory.getLogger(Darc.class);

    @BeforeAll
    static void initAll() throws CothorityCryptoException {
        owner = new SignerEd25519();
        user = new SignerEd25519();
        darc = new Darc(Arrays.asList(owner.getIdentity()), null, null);
    }

    @Test
    void rules() throws Exception {
        // List rules.
        String evolve = "invoke:evolve";
        assertEquals(evolve, darc.getActions().get(0));
        byte[] rule = darc.getExpression(evolve);

        // Change existing rule.
        darc.setRule(evolve, new byte[]{});
        assertFalse(Arrays.equals(rule, darc.getExpression(evolve)));

        // Add a new rule
        String spawn = "spawn:darc";
        byte[] spawnExression = String.format("%s | %s", owner.toString(), user.toString()).getBytes();
        darc.setRule(spawn, spawnExression);
        assertEquals(2, darc.getActions().size());
        assertArrayEquals(spawnExression, darc.getExpression(spawn));

        // Delete a rule
        byte[] oldExpression = darc.removeAction(spawn);
        assertArrayEquals(spawnExression, oldExpression);
    }
}