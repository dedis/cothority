package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

import javax.annotation.Nonnull;
import java.net.URI;
import java.util.ArrayList;

public class OcsFactory {
    private ArrayList<ServerIdentity> servers = new ArrayList<>();
    private SkipblockId genesis;

    /**
     * Set chain genesis (getId/hash of the fist block in the chain)
     *
     * @param genesis
     * @throws IllegalStateException when genesis can not be decoded or is too short
     */
    public OcsFactory setGenesis(final SkipblockId genesis) {
        this.genesis = genesis;
        return this;
    }

    /**
     * @param conode    cothority server address (base address in tcp://127.0.0.0:7001 form)
     * @param publicKey server public symmetricKey hex encoded to a string
     * @throws IllegalArgumentException when conode address is incorrect
     */
    public OcsFactory addConode(final URI conode, final String publicKey) {
        if (!conode.getScheme().equals("tcp")) {
            throw new IllegalArgumentException("conode address must be in tcp format like \"tcp://127.0.0.0:7001\"");
        }

        servers.add(new ServerIdentity(conode, publicKey));
        return this;
    }

    public OnchainSecrets createConnection() throws CothorityCommunicationException {
        if (null == genesis) {
            throw new IllegalStateException("Connection can not be established. No genesis specified.");
        }

        try {
            return new OnchainSecrets(createRoster(), genesis);
        } catch (CothorityCryptoException e) {
            throw new CothorityCommunicationException("Unable to connect to cothority ", e);
        }
    }

    /**
     * Create a new skipchain. New skipchain will be created and ID of genesis block will be returned.
     * To make other operations in the same skipchain you need to connect in normal way using skipblock ID.
     *
     * @return skipblock ID of a new genesis block
     */
    public SkipblockId initialiseNewSkipchain(@Nonnull Signer admin) throws CothorityCommunicationException {
        try {
            Roster roster = createRoster();

            Darc adminDarc = createAdminDarc(admin);
            OnchainSecrets ocs = new OnchainSecrets(roster, adminDarc);

            return ocs.getGenesis();
        } catch (CothorityCryptoException e) {
            throw new CothorityCommunicationException("Unable to create a new skipchain", e);
        }
    }

    private Roster createRoster() {
        if (servers.size() < 1) {
            throw new IllegalStateException("Connection can not be established. No cothority server was specified.");
        }
        return new Roster(servers);
    }
    private Darc createAdminDarc(@Nonnull Signer admin) throws CothorityCommunicationException {
        try {
            return new Darc(admin, null, null);
        } catch (CothorityCryptoException e) {
            throw new CothorityCommunicationException("Unable to create admin DARC for a new skipchain", e);
        }
    }
}
