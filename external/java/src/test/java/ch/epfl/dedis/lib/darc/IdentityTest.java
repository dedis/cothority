package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.proto.DarcProto;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public class IdentityTest {

    @Test
    void instantiateIdentity() throws Exception{
        SignerEd25519 owner = new SignerEd25519();
        SignerEd25519 user = new SignerEd25519();

        Identity ownerI = IdentityFactory.New(owner);
        Identity userI = IdentityFactory.New(user);

        assertNotEquals(ownerI.toProto(), userI.toProto());
        assertEquals(ownerI, IdentityFactory.New(ownerI.toProto()));
    }

    @Test
    void serialization() throws Exception{
        SignerEd25519 owner = new SignerEd25519();
        Identity ownerI = IdentityFactory.New(owner);
        DarcProto.Identity proto = ownerI.toProto();
        Identity ownerI2 = IdentityFactory.New(proto);

        assertTrue(ownerI.equals(ownerI2));
    }

    @Test
    void testKeycard() throws Exception{

    }
}
