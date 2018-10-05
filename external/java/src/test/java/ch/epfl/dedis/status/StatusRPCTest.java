package ch.epfl.dedis.status;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.ServerIdentity;
import org.junit.jupiter.api.Test;

class StatusRPCTest {
    @Test
    void getStatus() throws Exception {
        TestServerController testInstanceController = TestServerInit.getInstance();
        for (ServerIdentity sid: testInstanceController.getRoster().getNodes()) {
            // If the RPC call fails then an exception is thrown, test passes if there are no exceptions.
            StatusRPC.getStatus(sid);
        }
    }
}