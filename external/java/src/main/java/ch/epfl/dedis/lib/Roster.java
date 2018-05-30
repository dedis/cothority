package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.crypto.Ed25519;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.proto.RosterProto;
import com.google.protobuf.ByteString;

import java.util.ArrayList;
import java.util.List;

/**
 * dedis/lib
 * Roster.java
 * Purpose: A list of ServerIdentities make up a roster that can be used as a temporary
 * cothority.
 */

public class Roster {
    private List<ServerIdentity> nodes = new ArrayList<>();
    private Point aggregate; // TODO: can we find better name for it? like aggregatePublicKey or aggregatedKey?

    public Roster(List<ServerIdentity> servers) {
        nodes.addAll(servers);

        for (final ServerIdentity serverIdentity : nodes) {
            if (aggregate == null) {
                // TODO: it will be much better if there is some kind of 'zero' element for Ed25519Point type. Is it possible to use just a new created Ed25519Point
                aggregate = serverIdentity.Public;
            } else {
                aggregate = aggregate.add(serverIdentity.Public);
            }
        }
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

    public ByteString sendMessage(String path, com.google.protobuf.GeneratedMessageV3 proto) throws CothorityCommunicationException {
        // TODO - fetch a random node.
        return ByteString.copyFrom(nodes.get(0).SendMessage(path, proto.toByteArray()));
    }
}
