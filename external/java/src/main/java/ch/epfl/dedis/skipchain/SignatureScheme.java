package ch.epfl.dedis.skipchain;

/**
 * Enumeration of the list of available signature scheme
 */
public enum SignatureScheme {
    BLS(0),
    BDN(1);

    private int value;

    SignatureScheme(int value) {
        this.value = value;
    }

    public int getValue() {
        return value;
    }

    /**
     * Convert an integer into the correct enumeration if it exists, otherwise returns null
     * @param value The value to convert
     * @return the related enumeration or null
     */
    public static SignatureScheme fromValue(int value) {
        for (SignatureScheme scheme : values()) {
            if (scheme.value == value) {
                return scheme;
            }
        }

        return null;
    }
}
