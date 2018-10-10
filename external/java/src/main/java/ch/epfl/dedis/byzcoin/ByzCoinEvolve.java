package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.byzcoin.contracts.DarcInstance;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Rules;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityException;

/**
 * This class has only static methods and can be used to evolve the ByzCoin ledger over multiple versions.
 */
public class ByzCoinEvolve {
    /**
     * Things to evolve since this version:
     *   - update the genesis darc to include a 'invoke:update_config' rule.
     *
     * @param bc a running byzcoin ledger
     * @param admin allowed to call 'invoke:evolve' on the genesis darc.
     * @throws CothorityException
     */
    public static void from20181008(ByzCoinRPC bc, Signer admin) throws CothorityException{
        DarcInstance genesis = bc.getGenesisDarcInstance();
        Darc updatedGenesis = genesis.getDarc();
        updatedGenesis.addIdentity("invoke:update_config", admin.getIdentity(), Rules.OR);
        genesis.evolveDarcAndWait(updatedGenesis, admin, 20);
    }
}
