package ch.epfl.dedis.lib;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.network.ServerToml;
import ch.epfl.dedis.lib.proto.NetworkProto;
import ch.epfl.dedis.lib.proto.StatusProto;
import com.moandjiezana.toml.Toml;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.File;
import java.net.URL;

import static org.junit.jupiter.api.Assertions.*;

class ServerIdentityTest {
    private final static Logger logger = LoggerFactory.getLogger(ServerIdentityTest.class);
    private ServerIdentity si;

    @BeforeEach
    void initConodes() {
        TestServerController testServerController = TestServerInit.getInstance();
        si = testServerController.getMasterConode();
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
    void testProto(){
        NetworkProto.ServerIdentity si_proto = si.toProto();
        assertEquals(16, si_proto.getId().size());
    }

    @Test
    void testToml() throws Exception {
        ClassLoader loader = getClass().getClassLoader();
        URL filepath = loader.getResource("public.toml");
        assertNotNull(filepath);

        Toml toml = new Toml().read(new File(filepath.getFile()));
        ServerIdentity si = new ServerIdentity(toml.getTables("servers").get(0).to(ServerToml.class));
        assertNotNull(si);
        assertNotNull(si.getAddress());
        assertNotNull(si.getPublic());
        assertTrue(si.getServiceIdentities().size() > 0);
    }
}
