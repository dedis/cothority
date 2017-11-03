package com.byzgen.ocsimpl;

import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.ocs.OnchainSecrets;

import java.net.URI;
import java.util.ArrayList;
import java.util.Base64;

public class OcsFactory {
    private static final int MIN_ID_LEN = 32;
    private ArrayList<ServerIdentity> servers = new ArrayList<>();
    private byte[] genesis;

    private boolean initialiseNewChain = false;

    /**
     * Create a new chain.
     *
     * TODO: In the future initialisation of a chain will be secured and probably will be moved to different class
     *
     * For new initialised chain genesis is not required.
     *
     */
    public OcsFactory initialiseNewChain() {
        this.initialiseNewChain = true;
        return this;
    }

    /**
     * Set chain genesis (ID/hash of the fist block in the chain)
     *
     * @param genesisBase64
     * @throws IllegalStateException when genesis can not be decoded or is too short
     */
    public OcsFactory setGenesis(final String genesisBase64) {
        // TODO: I'd like to see this part as a small external class which will take care of IDs
        this.genesis = Base64.getDecoder().decode(genesisBase64);
        if ( this.genesis.length < MIN_ID_LEN) {
            throw new IllegalArgumentException("Genesis value is too short");
        }
        return this;
    }

    /**
     * @param conode cothority server address (base address in tcp://127.0.0.0:7001 form)
     * @param publicKey       server public symmetricKey base64 encoded to a string
     *
     * @throws IllegalArgumentException when conode address is incorrect
     *
     */
    public OcsFactory addConode(final URI conode, final String publicKey) {
        if (!conode.getScheme().equals("tcp")) {
            throw new IllegalArgumentException("conode address must be in tcp format like \"tcp://127.0.0.0:7001\"");
        }

        servers.add(new ServerIdentity(conode, publicKey));
        return this;
    }

    public OnchainSecrets createConnection() throws CothorityCommunicationException {
        if (servers.size() < 1) {
            throw new IllegalStateException("Connection can not be established. No cothority server was specified.");
        }

        if (null == genesis && !initialiseNewChain) {
            throw new IllegalStateException("Connection can not be established. No genesis specified.");
        }

        Roster roster = new Roster(servers);
        if (initialiseNewChain) {
            // TODO
            throw new CothorityCommunicationException("Need to implement darc here");
//            return new OnchainSecrets(roster);
        }
//        return new OnchainSecrets(roster, genesis);
        return null;
    }
}
