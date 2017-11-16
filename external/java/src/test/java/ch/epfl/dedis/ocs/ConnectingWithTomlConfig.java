package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.byzgen.OcsFactory;
import com.moandjiezana.toml.Toml;

import java.net.URI;
import java.net.URISyntaxException;
import java.util.List;
import java.util.stream.Collectors;

public class ConnectingWithTomlConfig {
    public static OnchainSecretsRPC connectClusterWithTomlConfig(String groupToml, Signer admin) throws CothorityCommunicationException {
        OcsFactory ocsFactory = new OcsFactory();

        Toml toml = new Toml().read(groupToml);
        List<Toml> servers = toml.getTables("servers");
        servers.stream().forEach(server -> ocsFactory.addConode(
                getServerURIFromToml(server), getPublicKeyFromToml(server)));

        SkipblockId skipblockId = ocsFactory.initialiseNewSkipchain(admin);

        ocsFactory.setGenesis(skipblockId);
        return ocsFactory.createConnection();
    }

    public static Roster constructRosterWithTomlConfig(String groupToml) {
        Toml toml = new Toml().read(groupToml);
        List<Toml> servers = toml.getTables("servers");
        List<ServerIdentity> nodes = servers.stream().map(server -> new ServerIdentity(getServerURIFromToml(server),
                getPublicKeyFromToml(server))).collect(Collectors.toList());

        return new Roster(nodes);
    }

    public static URI getServerURIFromToml(Toml t) {
        try {
            return new URI(t.getString("Address"));
        }
        catch (URISyntaxException e) {
            throw new IllegalArgumentException("Toml server definition is broken", e);
        }
    }

    public static String getPublicKeyFromToml(Toml t) {
        return t.getString("Point");
    }
}
