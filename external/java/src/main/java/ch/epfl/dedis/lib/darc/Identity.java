package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.proto.DarcProto;

public interface Identity {
    /**
     * Returns true if the verification of signature on the sha-256 of msg is
     * successful or false if not.
     * @param msg
     * @param signature
     * @return
     */
    boolean verify(byte[] msg, byte[] signature);

    /**
     * Creates a protobuf-representation of the implementation. The protobuf
     * representation has to hold all necessary fields to represent any of the
     * identity implementations.
     * @return
     */
    DarcProto.Identity toProto();

    boolean equals(Object other);

    String typeString();

    String toString();
}
