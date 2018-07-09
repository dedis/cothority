package ch.epfl.dedis.ocs;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.proto.NetworkProto;
import ch.epfl.dedis.proto.StatusProto;
import com.google.protobuf.ByteString;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import static org.junit.jupiter.api.Assertions.*;

class ServerIdentityTest {
    private final static Logger logger = LoggerFactory.getLogger(ServerIdentityTest.class);
    private TestServerController testServerController;
    private ServerIdentity si;

    @BeforeEach
    void initConodes() {
        testServerController = TestServerInit.getInstance();
        si = new ServerIdentity(testServerController.getMasterConode().getAddress(),
                testServerController.getMasterConode().getPublicKey());
    }

    @Test
    void testGetStatus() {
        try {
            StatusProto.Response resp = si.GetStatus();
            assertNotNull(resp);
        } catch (Exception e) {
            logger.error(e.getLocalizedMessage(), e);
            Assertions.fail("exception was not expected");
        }
    }

    @Test
    void testCreate() {
        // TODO: there is not much value in this test
        assertEquals(testServerController.getMasterConode().getAddress().toString(), si.getAddress().toString());
        assertNotEquals(null, si.Public);
    }

    @Test
    void testProto(){
        NetworkProto.ServerIdentity si_proto = si.toProto();
        byte[] id = Hex.parseHexBinary("482FB9CFC2B55AB68C5F811C1D47B9E1");
        assertArrayEquals(ByteString.copyFrom(id).toByteArray(), si_proto.getId().toByteArray());
    }
}
