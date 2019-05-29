package ch.epfl.dedis.lib.network;

import java.nio.ByteBuffer;

/**
 * The default handler interface for handling streaming responses.
 */
public interface StreamHandler {
    void receive(ByteBuffer message);

    void error(String s);
}
