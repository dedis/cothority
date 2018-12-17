package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.ByteBuffer;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;

/**
 * A factory to create point from different input. A protobuf input will have a tag at the beginning
 * to let the factory know which point to create and a point coming from a toml file will require the
 * suite information that must be available.
 */
public class PointFactory {
    static public final String SUITE_ED25519 = "Ed25519";
    static public final String SUITE_BN256 = "Bn256.adapter";

    static private final Logger logger = LoggerFactory.getLogger(Ed25519Point.class);
    static private final PointFactory INSTANCE = new PointFactory();

    /**
     * Get the singleton instance of the factory
     * @return the instance
     */
    static public PointFactory getInstance() {
        return INSTANCE;
    }

    private Map<Tag, PointGenerator> tags;

    /**
     * Instantiate the factory with bindings to Ed25519 and Bn256 points. More must be
     * added for new kind of points.
     */
    private PointFactory() {
        tags = new HashMap<>();
        tags.put(new Tag(Bn256G2Point.marshalID), Bn256G2Point::new);
        tags.put(new Tag(Ed25519Point.marshalID), Ed25519Point::new);
    }

    /**
     * Create a point using a protobuf input
     * @param pubkey the byte string of the public key
     * @return the point or null
     */
    public Point fromProto(ByteString pubkey) {
        byte[] buf = Arrays.copyOfRange(pubkey.toByteArray(), 0, 8);
        Tag tag = new Tag(buf);

        if (tags.containsKey(tag)) {
            try {
                return tags.get(tag).make(pubkey.toByteArray());
            } catch (CothorityCryptoException e) {
                logger.error(e.getMessage());
            }
        }

        return null;
    }

    /**
     * Create a point using a toml input string
     * @param suite the name of the suite to use to create the point
     * @param pubhex the hex string of the public key
     * @return the point or null
     */
    public Point fromToml(String suite, String pubhex) {
        try {
            switch (suite) {
                case PointFactory.SUITE_ED25519:
                    return new Ed25519Point(pubhex);
                case PointFactory.SUITE_BN256:
                    return new Bn256G2Point(pubhex);
                default:
                    return null;
            }
        } catch (CothorityCryptoException e) {
            logger.error(e.getMessage());
        }

        return null;
    }

    /**
     * Generator used to create point of the right suite for given tags
     */
    private interface PointGenerator {
        /**
         * Instantiate the point
         * @param data byte array of the point with the tag still prepended
         * @return the point
         */
        Point make(byte[] data) throws CothorityCryptoException;
    }

    /**
     * Tag used as a key to bind a generator to a point type
     */
    private class Tag {
        private byte[] tag;

        Tag(byte[] value) {
            tag = value;
        }

        @Override
        public boolean equals(Object other) {
            if (!(other instanceof Tag)) {
                return false;
            }

            return Arrays.equals(tag, ((Tag) other).tag);
        }

        @Override
        public int hashCode() {
            return ByteBuffer.wrap(tag).getInt();
        }
    }
}
