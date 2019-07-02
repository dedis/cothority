package ch.epfl.dedis.lib.network;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.UUIDType5;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.proto.NetworkProto;
import ch.epfl.dedis.lib.proto.OnetProto;
import com.google.protobuf.ByteString;
import com.moandjiezana.toml.Toml;

import java.net.URISyntaxException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.stream.Collectors;

/**
 * dedis/lib
 * Roster.java
 * Purpose: A list of ServerIdentities make up a roster that can be used as a temporary
 * cothority.
 */

public class Roster {
    private List<ServerIdentity> nodes = new ArrayList<>();
    private Point aggregate;

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
        if (nodes.size() > 0) {
            aggregate = nodes.get(0).getPublic().getZero();
        }
        for (final ServerIdentity serverIdentity : nodes) {
            aggregate = aggregate.add(serverIdentity.getPublic());
        }
    }

    /**
     * Get the ID of the roster
     *
     * @return the id as bytes
     */
    public UUID getID() {
        final MessageDigest digest;
        try {
            digest = MessageDigest.getInstance("SHA-256");
        } catch (NoSuchAlgorithmException e) {
            return null;
        }

        for (ServerIdentity id : nodes) {
            digest.update(id.getPublic().toBytes());

            for (ServiceIdentity srvid: id.getServiceIdentities()) {
                digest.update(srvid.getPublic().toBytes());
            }
        }

        byte[] h = digest.digest();
        return UUIDType5.nameUUIDFromNamespaceAndString(UUIDType5.NAMESPACE_URL, Hex.printHexBinary(h).toLowerCase());
    }

    public List<ServerIdentity> getNodes() {
        return nodes;
    }

    public List<Point> getServicePublics(String serviceName) {
        return this.nodes.stream()
                .map(sid -> sid.getServicePublic(serviceName))
                .collect(Collectors.toList());
    }

    public OnetProto.Roster toProto() {
        OnetProto.Roster.Builder r = OnetProto.Roster.newBuilder();
        r.setId(ByteString.copyFrom(UUIDType5.toBytes(getID())));
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
                // Special case: allow URL when it should be Url
                if (srvToml.contains("URL")) {
                    srv.Url = srvToml.getString("URL");
                }
                cothority.add(new ServerIdentity(srv));
            } catch (URISyntaxException e) {
                throw new RuntimeException(e);
            }
        }
        return new Roster(cothority);
    }

    @Override
    public String toString() {
        StringBuilder out = new StringBuilder();
        out.append("[");
        for (int i = 0; i < this.getNodes().size(); i++) {
            if (i != 0) {
                out.append(",");
            }
            out.append(this.getNodes().get(i).getAddress().toString());
        }
        out.append("]");
        return out.toString();
    }
}
