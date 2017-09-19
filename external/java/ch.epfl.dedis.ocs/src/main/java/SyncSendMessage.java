import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;

import java.net.URI;
import java.nio.ByteBuffer;
import java.util.concurrent.CountDownLatch;

public class SyncSendMessage {
    private ServerIdentity serverIdentity;
    public ByteBuffer response;
    public Boolean ok = false;
    public String error;

    public SyncSendMessage(ServerIdentity serverIdentity, String path, byte[] msg) throws Exception {
        this.serverIdentity = serverIdentity;
        final CountDownLatch statusLatch = new CountDownLatch(1);
        String uri = String.format("ws://%s/%s", serverIdentity.AddressWebSocket(), path);
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

    public class ErrorSendMessage extends Exception {
        public ErrorSendMessage(String message) {
            super(message);
            System.out.println(message);
        }
    }
}
