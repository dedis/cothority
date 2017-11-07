package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.proto.ServerIdentityProto;
import ch.epfl.dedis.proto.StatusProto;
import com.google.protobuf.ByteString;
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
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
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

    public StatusProto.Response GetStatus() throws Exception {
        StatusProto.Request request =
                StatusProto.Request.newBuilder().build();
        SyncSendMessage msg = new SyncSendMessage("Status/Request", request.toByteArray());
        if (msg.ok) {
            return StatusProto.Response.parseFrom(msg.response);
        } else {
            logger.warn("error sending message: " + msg.error);
        }

        return null;
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
        try {
            ServerIdentity.SyncSendMessage msg =
                    new ServerIdentity.SyncSendMessage(path, data);

            if (msg.ok) {
                return msg.response.array();
            } else {
                throw new CothorityCommunicationException("Error while sending message: " + msg.error);
            }
        } catch (Exception e) {
            throw new CothorityCommunicationException("Cothority communication error: " + e.getMessage(), e);
        }
    }

    public class SyncSendMessage {
        public ByteBuffer response;
        public Boolean ok = false;
        public String error;

        public SyncSendMessage(String path, byte[] msg) throws Exception {
            final CountDownLatch statusLatch = new CountDownLatch(1);
            WebSocketClient ws = new WebSocketClient(buildWebSocketAdddress(path)) {
                @Override
                public void onMessage(String msg) {
                    error = "This should never happen:" + msg;
                    statusLatch.countDown();
                }

                @Override
                public void onMessage(ByteBuffer message) {
                    try {
                        ok = true;
                        response = message;
                    } catch (Exception e) {
                        error = "Exception: " + e.toString();
                    }
                    statusLatch.countDown();
                }

                @Override
                public void onOpen(ServerHandshake handshake) {
                    this.send(msg);
                }

                @Override
                public void onClose(int code, String reason, boolean remote) {
                    logger.warn("closed connection: " + reason);
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
            if (!ok) {
                throw new ErrorSendMessage(error);
            }
        }

        public class ErrorSendMessage extends Exception {
            public ErrorSendMessage(String message) {
                super(message);
                logger.warn("error while sending message: " + message);
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
