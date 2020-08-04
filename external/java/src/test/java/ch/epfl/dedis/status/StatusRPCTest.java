package ch.epfl.dedis.status;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.network.ServerToml;
import com.moandjiezana.toml.Toml;
import org.junit.jupiter.api.Disabled;
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

    /**
     * getStatusProd is a unit test that talks to the production server, in order to test
     * websockets over TLS with a proper certificate.
     *
     * @throws Exception
     */
    @Disabled("getStatusProd disabled: unit tests should not depend on external servers")
    @Test
    void getStatusProd() throws Exception {
        String prodToml = "[[servers]]\n" +
                "  Address = \"tls://conode.dedis.ch:7000\"\n" +
                "  Url = \"https://conode.dedis.ch\"\n" +
                "  Suite = \"Ed25519\"\n" +
                "  Public = \"ec5c65a3c922d1df32075640e3de606197be24af76059a2ef145501122884bd3\"\n" +
                "  Description = \"EPFL Cothority-server\"\n" +
                "  [servers.Services]\n" +
                "    [servers.Services.ByzCoin]\n" +
                "      Public = \"6f69dc10dbef8f4d80072aa9d1bee191b0f68b137a9d06d006c39fe6667738fa2d3439caf428a1dcb6f4a5bd2ce6ff6f1462ebb1b7374080d95310bc6e1115e105d7ae38f9fed1585094b0cb13dc3a0f3e74daeaa794ca10058e44ef339055510f4d12a7234779f8db2e093dd8a14a03440a7d5a8ef04cac8fd735f20440b589\"\n" +
                "      Suite = \"bn256.adapter\"\n" +
                "    [servers.Services.PoPServer]\n" +
                "      Public = \"8f3d081c68394ffa6b6049da3f65ff996549ae4ccf9584a5a0b0ad6b7d6263265b39d9c044b2a58038670d6a8efe57dcc99a0ab7cbbd91dc08febacd4a1ee548142438b5eedca67789ba0bb664b02beea62cf40cde2d2a2f3794e9b3afdbacb322090b653b723ee59ae2d8b6db7281c32f764bc4250d160caab058057e25fa8a\"\n" +
                "      Suite = \"bn256.adapter\"\n" +
                "    [servers.Services.Skipchain]\n" +
                "      Public = \"32ba0cccec06ac4259b39102dcba13677eb385e0fdce99c93406542c5cbed3ec6ac71a81b01207451346402542923449ecf71fc0d69b1d019df34407b532fb2a09005c801e359afb377cc3255e918a096912bf6f7b7e4040532404996e05f78c408760b57fcf9e04c50eb7bc413438aca9d653dd0b6a8353d128370ebd4bdb10\"\n" +
                "      Suite = \"bn256.adapter\"\n" +
                "    [servers.Services.blsCoSiService]\n" +
                "      Public = \"6a62b35ee5ec659625bdcc69b47e14a5b5aad9a0aacb8c6ac1fa301667471be915da15f6fefa2537ee5cc8fdad0d31de01f3f7ab4dda80aa104215f1ee85f1e255cd767d8f353fd5f89815b18a8f0e96e08532a131f221e87d3e19eb07f0e27b55b03977579a30f8ce4aad04449f2ec405c4070cf37786de8322e8109d52b891\"\n" +
                "      Suite = \"bn256.adapter\"\n";

        Toml toml = new Toml().read(prodToml);
        ServerIdentity si = new ServerIdentity(toml.getTables("servers").get(0).to(ServerToml.class));
        StatusRPC.getStatus(si);
    }
}
