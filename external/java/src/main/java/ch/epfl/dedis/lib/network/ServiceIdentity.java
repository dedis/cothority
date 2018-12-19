package ch.epfl.dedis.lib.network;

import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.PointFactory;
import com.google.protobuf.ByteString;

/**
 * A service identity is used to store public information about a service of a conode like the key pair
 * used if it doesn't use the default key pair.
 */
public class ServiceIdentity {
    private final String name;
    private final Point point;
    private final String suite;

    /**
     * Create a service identity from the service name, the suite and the public
     * key in hexadecimal
     * @param name service name
     * @param suite crypto suite
     * @param pubkey public key
     */
    ServiceIdentity(String name, String suite, String pubkey) {
        this.name = name;
        this.point = PointFactory.getInstance().fromToml(suite, pubkey);
        this.suite = suite;
    }

    /**
     * Create a service identity from the service name and the marshal representation of the point
     * @param name service name
     * @param pubkey marshal representation of the point
     */
    ServiceIdentity(String name, String suite, ByteString pubkey) {
        this.name = name;
        this.point = PointFactory.getInstance().fromProto(pubkey);
        this.suite = suite;
    }

    /**
     * Get the service name
     * @return the service name
     */
    public String getName() {
        return name;
    }

    /**
     * Get the public key as a point
     * @return the point
     */
    public Point getPublic() {
        return this.point;
    }

    /**
     * Get the name of the suite compatible with the public key
     * @return the name
     */
    public String getSuite() {
        return suite;
    }
}
