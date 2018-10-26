package ch.epfl.dedis.integration;

import ch.epfl.dedis.byzgen.CalypsoFactory;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.status.StatusRPC;

import javax.annotation.Nonnull;
import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.util.List;
import java.util.stream.Collectors;

public abstract class TestServerController {
    protected static final String CONODE_PUB_1 = "d829a0790ffa8799e4bbd1bee8da0507c9166b665660baba72dd8610fca27cc1";
    protected static final String CONODE_PUB_2 = "d750a30daa44713d1a4b44ca4ef31142b3b53c0c36a558c0d610cc4108bb4ecb";
    protected static final String CONODE_PUB_3 = "7f47f33084c3ecc233f8b05b8f408bbd1c2e4a129aae126f92becacc73576bc7";
    protected static final String CONODE_PUB_4 = "8b25f8ac70b85b2e9aa7faf65507d4f7555af1c872240305117b7659b1e58a1e";
    protected static final String CONODE_PUB_5 = "9bb51090e716c717a7fba22b42beba4be13f28636c93cdfa52a7e20df2d950a2";
    protected static final String CONODE_PUB_6 = "d072695625c1938533b39f0fc69e3d1054bcabed3b560ea5c74e29e3cf6609f7";
    protected static final String CONODE_PUB_7 = "72642f4db36d8c25df04698ab16988c3ab3d798bb5d1d4a985e3e2ceb3ba0869";

    public static final ServerIdentity conode1 = new ServerIdentity(buildURI("tcp://localhost:7002"), CONODE_PUB_1);
    public static final ServerIdentity conode2 = new ServerIdentity(buildURI("tcp://localhost:7004"), CONODE_PUB_2);
    public static final ServerIdentity conode3 = new ServerIdentity(buildURI("tcp://localhost:7006"), CONODE_PUB_3);
    public static final ServerIdentity conode4 = new ServerIdentity(buildURI("tcp://localhost:7008"), CONODE_PUB_4);
    public static final ServerIdentity conode5 = new ServerIdentity(buildURI("tcp://localhost:7010"), CONODE_PUB_5);
    public static final ServerIdentity conode6 = new ServerIdentity(buildURI("tcp://localhost:7012"), CONODE_PUB_6);
    public static final ServerIdentity conode7 = new ServerIdentity(buildURI("tcp://localhost:7014"), CONODE_PUB_7);

    /**
     * Counts the number of conodes that are running by making a status request to all nodes in the roster. Note that it
     * will not include nodes that are not in the roster.
     */
    public int countRunningConodes() {
        int failures = 0;
        for (ServerIdentity sid : this.getRoster().getNodes()) {
            try {
                StatusRPC.getStatus(sid);
            } catch (CothorityCommunicationException e) {
                failures++;
            }
        }
        return getRoster().getNodes().size() - failures;
    }

    public abstract void startConode(int nodeNumber) throws InterruptedException, IOException;

    public abstract void killConode(int nodeNumber) throws IOException, InterruptedException;

    public abstract List<CalypsoFactory.ConodeAddress> getConodes();

    public Roster getRoster() {
        return new Roster(getConodes().stream()
                .map(conodeAddress -> new ServerIdentity(conodeAddress.getAddress(), conodeAddress.getPublicKey()))
                .collect(Collectors.toList()));
    }

    public CalypsoFactory.ConodeAddress getMasterConode() {
        return getConodes().get(0);
    }

    @Nonnull
    public static URI buildURI(String str) {
        try {
            return new URI(str);
        } catch (URISyntaxException e) {
            throw new IllegalStateException("Unable to setup test services", e);
        }
    }
}
