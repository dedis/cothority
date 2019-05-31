package ch.epfl.dedis.lib.network;

import ch.epfl.dedis.lib.UUIDType5;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.PointFactory;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.proto.NetworkProto;
import ch.epfl.dedis.lib.proto.StatusProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.websocket.Session;
import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.nio.ByteBuffer;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;

/**
 * dedis/lib
 * ServerIdentity.java
 * Purpose: The node-definition for a node in a cothority. It contains the IP-address
 * and a description.
 */
public class ServerIdentity {
    private Point pubkey;
    private List<ServiceIdentity> serviceIdentities;
    private final URI conodeAddress;
    private final Logger logger = LoggerFactory.getLogger(ServerIdentity.class);

    public ServerIdentity(final URI serverWsAddress, Point pubkey) {
        this.conodeAddress = serverWsAddress;
        this.pubkey = pubkey;
        this.serviceIdentities = new ArrayList<>();
    }

    public ServerIdentity(ServerToml toml) throws URISyntaxException {
        this(new URI(toml.Address), null);

        this.pubkey = PointFactory.getInstance().fromToml(toml.Suite, toml.Public);

        for (Map.Entry<String, ServiceToml> entry : toml.Services.entrySet()) {
            ServiceIdentity srvid = new ServiceIdentity(entry.getKey(), entry.getValue().Suite, entry.getValue().Public);
            this.serviceIdentities.add(srvid);
        }
    }

    public ServerIdentity(NetworkProto.ServerIdentity sid) throws URISyntaxException {
        this(new URI(sid.getAddress()), null);

        this.pubkey = PointFactory.getInstance().fromProto(sid.getPublic());
        this.serviceIdentities = sid.getServiceIdentitiesList().stream()
                .map(srvid -> new ServiceIdentity(srvid.getName(), srvid.getSuite(), srvid.getPublic()))
                .collect(Collectors.toList());
    }

    public URI getAddress() {
        return conodeAddress;
    }

    public Point getPublic() {
        return pubkey;
    }

    /**
     * Get the public key for the given service name. If the service name does not exist, null is returned.
     */
    public Point getServicePublic(String serviceName) {
        for (ServiceIdentity si : this.serviceIdentities) {
            if (si.getName().equals(serviceName)) {
                return si.getPublic();
            }
        }
        return null;
    }


    public List<ServiceIdentity> getServiceIdentities() {
        return serviceIdentities;
    }

    public StatusProto.Response GetStatus() throws CothorityCommunicationException {
        StatusProto.Request request =
                StatusProto.Request.newBuilder().build();
        try {
            URI uri = buildWebSocketAdddress("Status/Request");
            CompletableFuture<ByteBuffer> reply = SimpleWebSocket.send(uri, request.toByteArray());
            ByteBuffer buffer = reply.get();

            return StatusProto.Response.parseFrom(buffer);
        } catch (URISyntaxException | InterruptedException | ExecutionException | InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e.toString(), e);
        }
    }

    public NetworkProto.ServerIdentity toProto() {
        NetworkProto.ServerIdentity.Builder si =
                NetworkProto.ServerIdentity.newBuilder();
        si.setPublic(pubkey.toProto());
        String pubStr = "https://dedis.epfl.ch/id/" + pubkey.toString().toLowerCase();
        byte[] id = UUIDType5.toBytes(UUIDType5.nameUUIDFromNamespaceAndString(UUIDType5.NAMESPACE_URL, pubStr));
        si.setId(ByteString.copyFrom(id));
        si.setAddress(getAddress().toString());
        si.setDescription("");

        for (ServiceIdentity srvid : serviceIdentities) {
            NetworkProto.ServiceIdentity.Builder data = NetworkProto.ServiceIdentity.newBuilder();
            data.setPublic(srvid.getPublic().toProto());
            data.setName(srvid.getName());
            data.setSuite(srvid.getSuite());
            si.addServiceIdentities(data.build());
        }

        return si.build();
    }

    /**
     * Synchronously send a message.
     *
     * @param path The API endpoint.
     * @param data The request message.
     * @return The response.
     * @throws CothorityCommunicationException if something went wrong
     */
    public byte[] SendMessage(String path, byte[] data) throws CothorityCommunicationException {
        try {
            CompletableFuture<ByteBuffer> reply = SimpleWebSocket.send(buildWebSocketAdddress(path), data);
            ByteBuffer buf = reply.get();

            return buf.array();
        } catch (URISyntaxException | InterruptedException | ExecutionException e) {
            throw new CothorityCommunicationException("couldn't send message", e);
        }
    }

    /**
     * Make a streaming connection.
     *
     * @param path The API endpoint.
     * @param data The request message.
     * @param h    The handler for handling every response.
     * @return The streaming connection.
     * @throws CothorityCommunicationException if something went wrong
     */
    public StreamingConn MakeStreamingConnection(String path, byte[] data, StreamHandler h) throws CothorityCommunicationException {
        return new StreamingConn(path, data, h);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        ServerIdentity other = (ServerIdentity) o;
        return other.getAddress().equals(getAddress()) &&
                other.pubkey.equals(pubkey);
    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(pubkey.toBytes());
    }

    private URI buildWebSocketAdddress(final String servicePath) throws URISyntaxException {
        return new URI("ws",
                conodeAddress.getUserInfo(),
                conodeAddress.getHost(),
                conodeAddress.getPort() + 1, // client operation use higher port number
                servicePath.startsWith("/") ? servicePath : "/".concat(servicePath),
                conodeAddress.getQuery(),
                conodeAddress.getFragment());
    }

    @Override
    public String toString() {
        return "ServerIdentitiy {"
                + "\n\tAddress: " + conodeAddress.toString()
                + "\n\tPublic: " + pubkey.toString()
                + "\n\tServices: " + serviceIdentities.toString()
                + "\n}";
    }

    public class StreamingConn {
        private Session session;

        /**
         * Close the connection, note that this function is non-blocking, so calling isClosed immediately after calling
         * close might not return the desired result.
         */
        public void close() {
            try {
                session.close();
            } catch (IOException e) {
                logger.error("couldn't close the stream", e);
            }
        }

        /**
         * Checks whether the connection is open. Note that the close function is non-blocking, so this function might
         * not return true immediately after close is called.
         *
         * @return true if closed
         */
        public boolean isClosed() {
            return !session.isOpen();
        }

        /**
         * Create the streaming connection (non-blocking).
         *
         * @param path The API endpoint.
         * @param msg  The initial message.
         * @param h    The handler, which is called on every message.
         * @throws CothorityCommunicationException
         */
        private StreamingConn(String path, byte[] msg, StreamHandler h) throws CothorityCommunicationException {
            try {
                session = StreamWebSocket.send(buildWebSocketAdddress(path), msg, h);
            } catch (URISyntaxException e) {
                throw new CothorityCommunicationException(e.getMessage(), e);
            }
        }
    }
}
