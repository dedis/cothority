package ch.epfl.dedis.lib;

import ch.epfl.dedis.proto.OCSProto;
import ch.epfl.dedis.proto.ServerIdentityProto;
import ch.epfl.dedis.proto.StatusProto;
import com.google.protobuf.ByteString;

import java.net.URI;
import java.nio.ByteBuffer;
import java.util.Base64;
import java.util.concurrent.CountDownLatch;

import com.moandjiezana.toml.*;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;

import javax.xml.bind.DatatypeConverter;

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
    public String Address;
    public String Description;
    public Crypto.Point Public;

    public ServerIdentity(String definition) {
        this(new Toml().read(definition).getTables("servers").get(0));
    }

    public ServerIdentity(Toml t) {
        this.Address = t.getString("Address");
        this.Description = t.getString("Description");
        String pub = t.getString("Point");
        byte[] pubBytes = Base64.getDecoder().decode(pub);
        this.Public = new Crypto.Point(pubBytes);
    }

    public String AddressWebSocket() {
        String[] ipPort = Address.replaceFirst("^tcp://", "").split(":");
        int Port = Integer.valueOf(ipPort[1]);
        return String.format("%s:%d", ipPort[0], Port + 1);
    }

    public StatusProto.Response GetStatus() throws Exception {
        StatusProto.Request request =
                StatusProto.Request.newBuilder().build();
        SyncSendMessage msg = new SyncSendMessage("Status/Request", request.toByteArray());
        if (msg.ok) {
            return StatusProto.Response.parseFrom(msg.response);
        } else {
            System.out.println(msg.error);
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
        si.setAddress(Address);
        si.setDescription(Description);
        return si.build();
    }

    public byte[] SendMessage(String path, byte[] data) throws CothorityError{
        try {
            ServerIdentity.SyncSendMessage msg =
                    new ServerIdentity.SyncSendMessage(path, data);

            if (msg.ok) {
                return msg.response.array();
            } else {
                throw new CothorityError("Error while sending message: " + msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }


    public class SyncSendMessage {
        public ByteBuffer response;
        public Boolean ok = false;
        public String error;

        public SyncSendMessage(String path, byte[] msg) throws Exception {
            final CountDownLatch statusLatch = new CountDownLatch(1);
            String uri = String.format("ws://%s/%s", AddressWebSocket(), path);
            WebSocketClient ws = new WebSocketClient(new URI(uri)) {
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
                    System.out.println("closed connection: " + reason);
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
                System.out.println(message);
            }
        }
    }
}
