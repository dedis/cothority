package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.crypto.Ed25519;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.proto.RosterProto;
import com.google.protobuf.ByteString;
import com.moandjiezana.toml.Toml;

import java.net.URISyntaxException;
import java.util.ArrayList;
import java.util.List;

/**
 * dedis/lib
 * Roster.java
 * Purpose: A list of ServerIdentities make up a roster that can be used as a temporary
 * cothority.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */

public class Roster {
    private List<ServerIdentity> nodes = new ArrayList<>();
    private Point aggregate; // TODO: can we find better name for it? like aggregatePublicKey or aggregatedKey?

    public Roster(List<ServerIdentity> servers) {
        nodes.addAll(servers);

        for (final ServerIdentity serverIdentity : nodes) {
            if (aggregate == null) {
                // TODO: it will be much better if there is some kind of 'zero' element for Point type. Is it possible to use just a new created Point
                aggregate = serverIdentity.Public;
            } else {
                aggregate = aggregate.add(serverIdentity.Public);
            }
        }
    }

    public static Roster FromToml(String groupToml) {
        Toml toml = new Toml().read(groupToml);
        List<ServerIdentity> cothority = new ArrayList<>();
        List<Toml> servers = toml.getTables("servers");
        for (Toml s : servers) {
            try {
                cothority.add(new ServerIdentity(s));
            } catch (URISyntaxException e) {
            }
        }
        return new Roster(cothority);
    }

    public List<ServerIdentity> getNodes() {
        return nodes;
    }

    public RosterProto.Roster getProto() {
        RosterProto.Roster.Builder r = RosterProto.Roster.newBuilder();
        r.setId(ByteString.copyFrom(Ed25519.uuid4()));
        nodes.forEach(n -> r.addList(n.getProto()));
        r.setAggregate(aggregate.toProto());

        return r.build();
    }

    public ByteString SendMessage(String path, com.google.protobuf.GeneratedMessageV3 proto) throws CothorityCommunicationException {
        // TODO - fetch a random node.
        return ByteString.copyFrom(nodes.get(0).SendMessage(path, proto.toByteArray()));
    }
}
