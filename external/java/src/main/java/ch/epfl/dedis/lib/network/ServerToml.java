package ch.epfl.dedis.lib.network;

import java.util.Map;

/**
 * Toml representation of the public server configuration
 */
public class ServerToml {
    /**
     * Default public key of the server
     */
    String Public;
    /**
     * The suite of the default key
     */
    String Suite;
    /**
     * The address of the conode
     */
    String Address;
    /**
     * The address of the conode's webserver (optional; if not provided the webserver is
     * assumed to be on address:port+1.
     */
    String Url;
    /**
     * The list of service configurations containing the public key
     */
    Map<String, ServiceToml> Services;
}
