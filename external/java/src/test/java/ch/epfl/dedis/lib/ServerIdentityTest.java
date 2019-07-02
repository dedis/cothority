package ch.epfl.dedis.lib;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.network.ServerToml;
import ch.epfl.dedis.lib.proto.NetworkProto;
import ch.epfl.dedis.lib.proto.StatusProto;
import com.moandjiezana.toml.Toml;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.File;
import java.net.URL;
import java.nio.ByteBuffer;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

class ServerIdentityTest {
    private final static Logger logger = LoggerFactory.getLogger(ServerIdentityTest.class);
    private ServerIdentity si;
    private ServerIdentity offline;

    @BeforeEach
    void initConodes() {
        TestServerController testServerController = TestServerInit.getInstance();
        si = testServerController.getMasterConode();
        offline = testServerController.getConodes().get(3);
    }

    @AfterAll
    static void startConodes() throws Exception {
        TestServerInit.getInstance().startConode(4);
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

    @Test
    void testSendMessage() throws Exception {
        CothorityCommunicationException thrown = assertThrows(CothorityCommunicationException.class,
                () -> si.SendMessage("Status/abc", new byte[0]));

        // Assert the websocket has been closed because of the correct reason
        assertTrue(thrown.getMessage().contains("The requested message hasn't been registered"));

        TestServerInit.getInstance().killConode(4);
        assertThrows(CothorityCommunicationException.class, () -> offline.GetStatus());
    }

    @Test
    void testMakeStream() throws Exception {
        CompletableFuture<Boolean> future = new CompletableFuture<>();
        ServerIdentity.StreamHandler h = new ServerIdentity.StreamHandler() {
            @Override
            public void receive(ByteBuffer message) {
                future.completeExceptionally(new Throwable("should not receive"));
            }

            @Override
            public void error(String s) {
                if (s.contains("The requested message hasn't been registered")) {
                    future.complete(true);
                } else {
                    future.completeExceptionally(new Throwable(s));
                }
            }
        };

        ServerIdentity.StreamingConn conn = si.MakeStreamingConnection("Status/abc", new byte[0], h);

        // throws if it completes with an exception that we don't expect
        future.get(2, TimeUnit.SECONDS);

        // It's closing asynchronously but it shouldn't take more a few seconds
        for (int i = 0; i < 10; i++) {
            if (conn.isClosed()) {
                return;
            }

            Thread.sleep(100);
        }

        fail("connection didn't close in time");
    }
}
