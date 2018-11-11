package ch.epfl.dedis.integration;

import ch.epfl.dedis.byzgen.CalypsoFactory;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.calypso.CalypsoRPC;
import ch.epfl.dedis.lib.exception.CothorityException;
import com.moandjiezana.toml.Toml;

import java.net.URI;
import java.net.URISyntaxException;
import java.util.List;
import java.util.stream.Collectors;

public class ConnectingWithTomlConfig {
    public static CalypsoRPC connectClusterWithTomlConfig(String groupToml, Signer admin) throws CothorityException {
        CalypsoFactory calypsoFactory = new CalypsoFactory();

        Toml toml = new Toml().read(groupToml);
        List<Toml> servers = toml.getTables("servers");
        servers.stream().forEach(server -> calypsoFactory.addConode(
                getServerURIFromToml(server), getPublicKeyFromToml(server)));

        CalypsoRPC crpc = calypsoFactory.initialiseNewCalypso(admin);

        calypsoFactory.setGenesis(crpc.getGenesisBlock().getSkipchainId());
        calypsoFactory.setLTSId(crpc.getLTSId());
        return calypsoFactory.createConnection();
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
        return t.getString("Ed25519Point");
    }
}
