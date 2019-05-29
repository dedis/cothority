package ch.epfl.dedis.lib.network;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.websocket.*;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.net.URI;
import java.nio.ByteBuffer;

/**
 * WebSocket client that will stream incoming messages through the given StreamHandler. It
 * will stop if an error occurred or if the session is closed.
 */
public class StreamWebSocket extends Endpoint {
    private final Logger logger = LoggerFactory.getLogger(StreamWebSocket.class);

    /**
     * Open a websocket connection and send the message. It will then listen for
     * incoming messages and send them through the stream handler.
     *
     * @param path  URI of the server
     * @param msg   The message to send after opening the connection
     * @param h     The stream handler
     * @return a websocket session that can be closed
     */
    static Session send(URI path, byte[] msg, StreamHandler h)
            throws IOException, DeploymentException {

        StreamWebSocket ws = new StreamWebSocket(h);

        ClientEndpointConfig cfg = ClientEndpointConfig.Builder.create().build();

        WebSocketContainer c = ContainerProvider.getWebSocketContainer();
        Session s = c.connectToServer(ws, cfg, path);
        s.getBasicRemote().sendBinary(ByteBuffer.wrap(msg));

        return s;
    }

    private StreamHandler h;
    private ByteArrayOutputStream stream = new ByteArrayOutputStream();

    private StreamWebSocket(StreamHandler handler) {
        h = handler;
    }

    @Override
    public void onOpen(Session session, EndpointConfig endpointConfig) {
        session.addMessageHandler(ByteBuffer.class, (data, last) -> {
            stream.write(data.array(), 0, data.capacity());

            if (last) {
                ByteBuffer buffer = ByteBuffer.wrap(stream.toByteArray());

                h.receive(buffer);
                // clean for the next message
                stream.reset();
            }
        });
    }

    @Override
    public void onClose(Session session, CloseReason reason) {
        if (reason.getCloseCode() != CloseReason.CloseCodes.NORMAL_CLOSURE) {
            h.error("websocket closed with error: "+reason.getReasonPhrase());
        }
    }

    @Override
    public void onError(Session session, Throwable throwable) {
        h.error(throwable.getMessage());

        try {
            session.close();
        } catch (IOException e) {
            logger.error("couldn't close the session", e);
        }
    }
}
