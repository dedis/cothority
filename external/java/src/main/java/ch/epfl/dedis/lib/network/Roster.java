package ch.epfl.dedis.lib.network;

import ch.epfl.dedis.lib.crypto.Ed25519;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.NetworkProto;
import ch.epfl.dedis.lib.proto.OnetProto;
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
 */

public class Roster {
    private List<ServerIdentity> nodes = new ArrayList<>();
    private Point aggregate; // TODO: can we find better name for it? like aggregatePublicKey or aggregatedKey?

    public Roster(List<ServerIdentity> servers) {
        nodes.addAll(servers);
        this.updateAggregate();
    }

    public Roster(OnetProto.Roster roster) throws URISyntaxException {
        List<ServerIdentity> sids = new ArrayList<>();
        for (NetworkProto.ServerIdentity sid : roster.getListList()) {
            sids.add(new ServerIdentity(sid));
        }
        nodes.addAll(sids);
        this.updateAggregate();
    }

    private void updateAggregate() {
        for (final ServerIdentity serverIdentity : nodes) {
            if (aggregate == null) {
                // TODO: it will be much better if there is some kind of 'zero' element for Ed25519Point type. Is it possible to use just a new created Ed25519Point
                aggregate = serverIdentity.getPublic();
            } else {
                aggregate = aggregate.add(serverIdentity.getPublic());
            }
        }
    }

    public List<ServerIdentity> getNodes() {
        return nodes;
    }

    public OnetProto.Roster toProto() {
        OnetProto.Roster.Builder r = OnetProto.Roster.newBuilder();
        r.setId(ByteString.copyFrom(Ed25519.uuid4()));
        nodes.forEach(n -> r.addList(n.toProto()));
        r.setAggregate(aggregate.toProto());

        return r.build();
    }

    /**
     * Synchronously sends a message.
     *
     * @param path  The API endpoint.
     * @param proto The protobuf encoded request.
     * @return the response
     * @throws CothorityCommunicationException if something went wrong
     */
    public ByteString sendMessage(String path, com.google.protobuf.GeneratedMessageV3 proto) throws CothorityCommunicationException {
        // TODO - fetch a random node.
        return ByteString.copyFrom(nodes.get(0).SendMessage(path, proto.toByteArray()));
    }

    /**
     * Sends a request to initialise a streaming connection.
     *
     * @param path  The API endpoint, note that this endpoint must support streaming (registered using RegisterStreamingRequest in the Go side).
     * @param proto The protobuf encoded request.
     * @param h     The handler for handling responses.
     * @return the streaming connection.
     * @throws CothorityCommunicationException if something went wrong
     */
    public ServerIdentity.StreamingConn makeStreamingConn(String path, com.google.protobuf.GeneratedMessageV3 proto, ServerIdentity.StreamHandler h) throws CothorityCommunicationException {
        // TODO - fetch a random node.
        return nodes.get(0).MakeStreamingConnection(path, proto.toByteArray(), h);
    }

    public static Roster FromToml(String groupToml) {
        Toml toml = new Toml().read(groupToml);
        List<ServerIdentity> cothority = new ArrayList<>();
        List<Toml> servers = toml.getTables("servers");

        for (Toml srvToml : servers) {
            try {
                ServerToml srv = srvToml.to(ServerToml.class);
                cothority.add(new ServerIdentity(srv));
            } catch (URISyntaxException e) {
                throw new RuntimeException(e);
            }
        }
        return new Roster(cothority);
    }
}
