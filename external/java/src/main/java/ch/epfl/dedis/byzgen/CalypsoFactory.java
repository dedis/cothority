package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.calypso.CalypsoRPC;
import ch.epfl.dedis.calypso.LTSId;
import ch.epfl.dedis.calypso.LTSInstance;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Rules;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.network.ServerIdentity;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

public class CalypsoFactory {
    private ArrayList<ServerIdentity> servers = new ArrayList<>();
    private SkipblockId genesis;
    private LTSId ltsId;

    private final static Logger logger = LoggerFactory.getLogger(CalypsoFactory.class);

    /**
     * Set chain genesis (getId/hash of the fist block in the chain)
     *
     * @param genesis the skipblock ID of the genesis block
     * @throws IllegalStateException when genesis can not be decoded or is too short
     * @return the factory
     */
    public CalypsoFactory setGenesis(final SkipblockId genesis) {
        this.genesis = genesis;
        return this;
    }

    /**
     * Sets the LTSId to use for all write and read requests.
     *
     * @param ltsId the lts id to use
     * @return CalypsoFactory for chaining
     */
    public CalypsoFactory setLTSId(final LTSId ltsId) {
        this.ltsId = ltsId;
        return this;
    }

    public CalypsoFactory addConode(ServerIdentity id) {
        if (!id.getAddress().getScheme().equals("tls")) {
            throw new IllegalArgumentException("conode address must be in tls:// format like \"tls://127.0.0.0:7771\"");
        }

        servers.add(id);
        return this;
    }

    /**
     * @param ids server identities
     * @throws IllegalArgumentException when conode address is incorrect
     * @return the factory
     */
    public CalypsoFactory addConodes(List<ServerIdentity> ids) {
        for (ServerIdentity id : ids) {
            // sanity check is done by this function
            addConode(id);
        }

        return this;
    }

    public CalypsoRPC createConnection() throws CothorityException {
        if (null == genesis) {
            throw new IllegalStateException("Connection can not be established. No genesis specified.");
        }
        if (null == ltsId) {
            throw new IllegalStateException("Connection can not be established. No ltsId specified.");
        }

        return CalypsoRPC.fromCalypso(createRoster(), genesis, ltsId);
    }

    /**
     * Create a new skipchain. New skipchain will be created and ID of genesis block will be returned.
     * To make other operations in the same skipchain you need to connect in normal way using skipblock ID.
     *
     * @param admin the Signer who will be the admin
     * @return skipblock ID of a new genesis block
     * @throws CothorityException if something goes wrong
     */
    public CalypsoRPC initialiseNewCalypso(Signer admin) throws CothorityException {
        Roster roster = createRoster();
        Darc adminDarc = ByzCoinRPC.makeGenesisDarc(admin, roster);
        adminDarc.addIdentity("invoke:" + LTSInstance.InvokeCommand, admin.getIdentity(), Rules.OR);
        adminDarc.addIdentity("spawn:" + LTSInstance.ContractId, admin.getIdentity(), Rules.OR);
        ByzCoinRPC bc = new ByzCoinRPC(roster, adminDarc, Duration.ofMillis(5000));
        for (ServerIdentity si: bc.getRoster().getNodes()
             ) {
            CalypsoRPC.authorise(si, bc.getGenesisBlock().getId());
        }
        return new CalypsoRPC(bc, adminDarc.getId(), roster,
                Collections.singletonList(admin), Collections.singletonList(1L));
    }

    private Roster createRoster() {
        if (servers.size() < 1) {
            throw new IllegalStateException("Connection can not be established. No cothority server was specified.");
        }
        return new Roster(servers);
    }
}
