package ch.epfl.dedis.integration;

import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.network.ServerToml;
import ch.epfl.dedis.status.StatusRPC;
import com.moandjiezana.toml.Toml;

import javax.annotation.Nonnull;
import java.io.File;
import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.URL;
import java.util.ArrayList;
import java.util.List;

public abstract class TestServerController {
    private List<ServerIdentity> identities;

    public TestServerController() {
        identities = new ArrayList<>();

        try {
            parseServerIdentities();
        } catch (Exception e) {
            System.out.println("Oops: ");
            e.printStackTrace();
        }
    }

    public List<ServerIdentity> getIdentities() {
        return identities;
    }

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

    public abstract void cleanDBs() throws IOException, InterruptedException;

    public abstract List<ServerIdentity> getConodes();

    public Roster getRoster() {
        return new Roster(getConodes());
    }

    public ServerIdentity getMasterConode() {
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

    /**
     * Read the public.toml file in the test resources and create the server identities from it
     *
     * @throws IOException        for file errors
     * @throws URISyntaxException for conode address errors
     */
    private void parseServerIdentities() throws IOException, URISyntaxException {
        ClassLoader loader = getClass().getClassLoader();
        URL filepath = loader.getResource("public.toml");
        if (filepath == null) {
            throw new IOException("missing public.toml file in the test resources");
        }

        File file = new File(filepath.getFile());
        Toml toml = new Toml().read(file);

        for (Toml srvToml : toml.getTables("servers")) {
            ServerToml srv = srvToml.to(ServerToml.class);

            identities.add(new ServerIdentity(srv));
        }
    }
}
