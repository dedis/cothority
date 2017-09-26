package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.CothorityCommunicationException;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import com.moandjiezana.toml.Toml;

import java.net.URI;
import java.net.URISyntaxException;
import java.util.List;
import java.util.stream.Collectors;

public class ConnectingWithTomlConfig {
    public static OnchainSecrets connectClusterWithTomlConfig(String groupToml) throws CothorityCommunicationException {
        OcsFactory ocsFactory = new OcsFactory();

        Toml toml = new Toml().read(groupToml);
        List<Toml> servers = toml.getTables("servers");
        servers.stream().forEach(server -> ocsFactory.addConode(
                getServerURIFromToml(server), getPublicKeyFromToml(server)));

        ocsFactory.initialiseNewChain();
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
        final String address = t.getString("Address");
        final String host = address.replaceFirst("^tcp://", "").split(":")[0];
        int port = 1 + Integer.valueOf(address.replaceFirst("^tcp://", "").split(":")[1]);

        try {
            return new URI("ws", null, host, port, null, null, null);
        }
        catch (URISyntaxException e) {
            throw new IllegalArgumentException("Toml server definition is broken", e);
        }
    }

    public static String getPublicKeyFromToml(Toml t) {
        return t.getString("Point");
    }
}
