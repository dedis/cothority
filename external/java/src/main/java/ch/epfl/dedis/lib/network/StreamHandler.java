package ch.epfl.dedis.lib.network;

import java.nio.ByteBuffer;

/**
 * The default handler interface for handling streaming responses.
 */
public interface StreamHandler {
    /**
     * Called when a message is correctly received from the stream channel
     * @param message The received message
     */
    void receive(ByteBuffer message);

    /**
     * Called if an error occurred
     * @param err Error string message
     */
    void error(String err);
}
