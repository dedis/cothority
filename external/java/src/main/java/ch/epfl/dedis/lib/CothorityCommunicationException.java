package ch.epfl.dedis.lib;

import com.google.protobuf.InvalidProtocolBufferException;

public class CothorityCommunicationException extends CothorityException {
    public CothorityCommunicationException(String message) {
        super(message);
    }
    public CothorityCommunicationException(String message, Throwable cause) { super(message, cause);}

    public CothorityCommunicationException(InvalidProtocolBufferException protobufProtocolException) {
        super(protobufProtocolException.getMessage(), protobufProtocolException);
    }
}
