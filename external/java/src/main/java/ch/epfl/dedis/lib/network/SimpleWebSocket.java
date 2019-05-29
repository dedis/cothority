package ch.epfl.dedis.lib.network;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.websocket.*;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.net.URI;
import java.nio.ByteBuffer;
import java.util.concurrent.CompletableFuture;

/**
 * WebSocket client for a one-shot message that will return the reply
 * inside a future that will either complete with the bytes or with an
 * exception.
 */
public class SimpleWebSocket extends Endpoint {
    private final Logger logger = LoggerFactory.getLogger(SimpleWebSocket.class);

    private static final long WS_IDLE_TIMEOUT = 20 * 1000; // 20 seconds

    /**
     * Open a websocket connection and send the message. It will listen for the
     * reply until the IDLE timeout is reached.
     *
     * @param path  URI of the server
     * @param msg   The message to send
     * @return a future that will complete with the reply or an exception
     */
    static CompletableFuture<ByteBuffer> send(URI path, byte[] msg)
            throws IOException, DeploymentException {

        SimpleWebSocket ws = new SimpleWebSocket();

        ClientEndpointConfig cfg = ClientEndpointConfig.Builder.create().build();

        WebSocketContainer c = ContainerProvider.getWebSocketContainer();
        Session s = c.connectToServer(ws, cfg, path);
        s.getBasicRemote().sendBinary(ByteBuffer.wrap(msg));

        return ws.future;
    }

    private ByteArrayOutputStream stream = new ByteArrayOutputStream();
    private CompletableFuture<ByteBuffer> future = new CompletableFuture<>();

    private SimpleWebSocket() {}

    @Override
    public void onOpen(Session session, EndpointConfig endpointConfig) {
        session.setMaxIdleTimeout(WS_IDLE_TIMEOUT);

        session.addMessageHandler(ByteBuffer.class, (data, last) -> {
            stream.write(data.array(), 0, data.capacity());

            if (last) {
                ByteBuffer buffer = ByteBuffer.wrap(stream.toByteArray());

                future.complete(buffer);

                try {
                    session.close();
                } catch (IOException e) {
                    logger.error("Couldn't close the session", e);
                }
            }
        });
    }

    @Override
    public void onClose(Session session, CloseReason reason) {
        if (reason.getCloseCode() != CloseReason.CloseCodes.NORMAL_CLOSURE) {
            future.completeExceptionally(new Throwable("websocket closed with error: "+reason.getReasonPhrase()));
        }
    }

    @Override
    public void onError(Session session, Throwable throwable) {
        future.completeExceptionally(throwable);
        try {
            session.close();
        } catch (IOException e) {
            logger.error("Couldn't close the session", e);
        }
    }
}
