import net.i2p.crypto.eddsa.spec.EdDSANamedCurveTable;
import net.i2p.crypto.eddsa.spec.EdDSAPublicKeySpec;
import net.i2p.crypto.eddsa.EdDSAPublicKey;

import java.net.URI;
import java.nio.ByteBuffer;
import java.security.PublicKey;
import java.util.Base64;
import java.util.concurrent.CountDownLatch;

import com.moandjiezana.toml.*;
import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;
import proto.StatusProto;


public class ServerIdentity {
    public String Address;
    public String Description;
    public PublicKey Public;

    public ServerIdentity(String definition) {
        this(new Toml().read(definition).getTables("servers").get(0));
    }

    public ServerIdentity(Toml t) {
        this.Address = t.getString("Address");
        this.Description = t.getString("Description");
        String pub = t.getString("Public");
        byte[] pubBytes = Base64.getDecoder().decode(pub);
        EdDSAPublicKeySpec spec = new EdDSAPublicKeySpec(pubBytes,
                EdDSANamedCurveTable.getByName("ed25519"));
        this.Public = new EdDSAPublicKey(spec);
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

    public class ErrorSendMessage extends Exception {
        public ErrorSendMessage(String message) {
            super(message);
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
            if (!ok){
                throw new ErrorSendMessage(error);
            }
        }
    }
}
