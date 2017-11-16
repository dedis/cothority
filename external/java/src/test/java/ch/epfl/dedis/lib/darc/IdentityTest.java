package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public class IdentityTest {

    @Test
    void instantiateIdentity() throws Exception{
        Ed25519Signer owner = new Ed25519Signer();
        Ed25519Signer user = new Ed25519Signer();

        Identity ownerI = IdentityFactory.New(owner);
        Identity userI = IdentityFactory.New(user);

        assertNotEquals(ownerI.toProto(), userI.toProto());
        assertEquals(ownerI, IdentityFactory.New(ownerI.toProto()));
    }

    @Test
    void serialization() throws Exception{
        Ed25519Signer owner = new Ed25519Signer();
        Identity ownerI = IdentityFactory.New(owner);
        DarcProto.Identity proto = ownerI.toProto();
        Identity ownerI2 = IdentityFactory.New(proto);

        assertTrue(ownerI.equals(ownerI2));
    }
}
