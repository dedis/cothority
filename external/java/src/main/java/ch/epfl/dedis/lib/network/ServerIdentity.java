package ch.epfl.dedis.lib.network;

import ch.epfl.dedis.lib.UUIDType5;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.PointFactory;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.proto.NetworkProto;
import ch.epfl.dedis.lib.proto.StatusProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.net.URI;
import java.net.URISyntaxException;
import java.nio.ByteBuffer;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
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

    public List<ServiceIdentity> getServiceIdentities() {
        return serviceIdentities;
    }

    public StatusProto.Response GetStatus() throws CothorityCommunicationException {
        StatusProto.Request request =
                StatusProto.Request.newBuilder().build();
        try {
            SyncSendMessage msg = new SyncSendMessage("Status/Request", request.toByteArray());
            return StatusProto.Response.parseFrom(msg.response);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e.toString());
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
        SyncSendMessage ssm = new SyncSendMessage(path, data);
        if (ssm.response == null) {
            throw new CothorityCommunicationException("Error while retrieving response - try again. Error-string is: " + ssm.error);
        }
        return ssm.response.array();
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
                + "\n\tAddress: "+conodeAddress.toString()
                + "\n\tPublic: "+pubkey.toString()
                + "\n\tServices: "+serviceIdentities.toString()
                + "\n}";
    }

    /**
     * The default handler interface for handling streaming responses.
     */
    public interface StreamHandler {
        void receive(ByteBuffer message);

        void error(String s);
    }

    public class StreamingConn {
        private WebSocketClient ws;

        /**
         * Close the connection, note that this function is non-blocking, so calling isClosed immediately after calling
         * close might not return the desired result.
         */
        public void close() {
            ws.close();
        }

        /**
         * Checks whether the connection is open. Note that the close function is non-blocking, so this function might
         * not return true immediately after close is called.
         *
         * @return true if closed
         */
        public boolean isClosed() {
            return ws.isClosed();
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
                ws = new WebSocketClient(buildWebSocketAdddress(path)) {
                    @Override
                    public void onMessage(String msg) {
                        logger.error("received a string msg, this should not happen on an honest server");
                        h.error("received a string msg, this should not happen on an honest server: " + msg);
                    }

                    @Override
                    public void onMessage(ByteBuffer message) {
                        h.receive(message);
                    }

                    @Override
                    public void onOpen(ServerHandshake handshake) {
                        this.send(msg);
                    }

                    @Override
                    public void onClose(int code, String reason, boolean remote) {
                        if (!reason.equals("")) {
                            h.error(reason);
                        }
                    }

                    @Override
                    public void onError(Exception ex) {
                        h.error(ex.toString());
                    }
                };

                ws.connect();
            } catch (URISyntaxException e) {
                throw new CothorityCommunicationException(e.getMessage());
            }
        }
    }

    public class SyncSendMessage {
        public ByteBuffer response;
        public String error;

        // TODO we're creating a new connection for every message, it's better to re-use connections
        private SyncSendMessage(String path, byte[] msg) throws CothorityCommunicationException {
            final CountDownLatch statusLatch = new CountDownLatch(1);
            try {
                WebSocketClient ws = new WebSocketClient(buildWebSocketAdddress(path)) {
                    @Override
                    public void onMessage(String msg) {
                        error = "This should never happen:" + msg;
                        statusLatch.countDown();
                    }

                    @Override
                    public void onMessage(ByteBuffer message) {
                        response = message;
                        close();
                        statusLatch.countDown();
                    }

                    @Override
                    public void onOpen(ServerHandshake handshake) {
                        this.send(msg);
                    }

                    @Override
                    public void onClose(int code, String reason, boolean remote) {
                        if (!reason.equals("")) {
                            error = reason;
                        }
                        statusLatch.countDown();
                    }

                    @Override
                    public void onError(Exception ex) {
                        error = "Error: " + ex.toString();
                        statusLatch.countDown();
                    }
                };

                // open websocket and send message.
                ws.connect();
                // wait for error or message returned.
                statusLatch.await();
            } catch (InterruptedException | URISyntaxException e) {
                throw new CothorityCommunicationException(e.toString());
            }
            if (error != null) {
                logger.error("error: {}", error);
                throw new CothorityCommunicationException(error);
            }
        }
    }
}
