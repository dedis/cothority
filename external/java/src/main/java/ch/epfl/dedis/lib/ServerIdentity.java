package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.proto.ServerIdentityProto;
import ch.epfl.dedis.proto.StatusProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import com.moandjiezana.toml.Toml;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.net.URI;
import java.net.URISyntaxException;
import java.nio.ByteBuffer;
import java.util.Base64;
import java.util.concurrent.CountDownLatch;

/**
 * dedis/lib
 * ServerIdentity.java
 * Purpose: The node-definition for a node in a cothority. It contains the IP-address
 * and a description.
 */

public class ServerIdentity {
    private final URI conodeAddress;
    public Point Public;
    private final Logger logger = LoggerFactory.getLogger(ServerIdentity.class);

    public ServerIdentity(final URI serverWsAddress, final String publicKey) {
        this.conodeAddress = serverWsAddress;
        // TODO: It will be better to use some class for server key and move this conversion outside of this class
        this.Public = new Point(Base64.getDecoder().decode(publicKey));
    }

    public ServerIdentity(Toml siToml) throws URISyntaxException {
        this(new URI(siToml.getString("Address")), siToml.getString("Point"));
    }

    public URI getAddress() {
        return conodeAddress;
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

    public ServerIdentityProto.ServerIdentity getProto() {
        ServerIdentityProto.ServerIdentity.Builder si =
                ServerIdentityProto.ServerIdentity.newBuilder();
        si.setPublic(Public.toProto());
        String pubStr = "https://dedis.epfl.ch/id/" + Public.toString().toLowerCase();
        byte[] id = UUIDType5.toBytes(UUIDType5.nameUUIDFromNamespaceAndString(UUIDType5.NAMESPACE_URL, pubStr));
        si.setId(ByteString.copyFrom(id));
        si.setAddress(getAddress().toString());
        si.setDescription("");
        return si.build();
    }

    public byte[] SendMessage(String path, byte[] data) throws CothorityCommunicationException {
        return new SyncSendMessage(path, data).response.array();
    }

    public class SyncSendMessage {
        public ByteBuffer response;
        public String error;

        public SyncSendMessage(String path, byte[] msg) throws CothorityCommunicationException {
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
                        if (reason != "") {
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
            } catch (InterruptedException e) {
                throw new CothorityCommunicationException(e.toString());
            } catch (URISyntaxException e) {
                throw new CothorityCommunicationException(e.toString());
            }
            if (error != null) {
                throw new CothorityCommunicationException(error);
            }
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
    }
}
