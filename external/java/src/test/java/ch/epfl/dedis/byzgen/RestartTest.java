package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.skipchain.ForwardLink;
import ch.epfl.dedis.skipchain.SkipchainRPC;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import static ch.epfl.dedis.byzcoin.ByzCoinRPCTest.BLOCK_INTERVAL;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;


public class RestartTest {
    Roster roster;
    static String bcStr = "";
    private final static Logger logger = LoggerFactory.getLogger(RestartTest.class);

    @BeforeEach
    void init() {
        TestServerController testServerController = TestServerInit.getInstance();
        roster = testServerController.getRoster();
    }

    @Test
    void Step1() throws CothorityException {
        Signer admin = new SignerEd25519();
        Darc adminDarc = ByzCoinRPC.makeGenesisDarc(admin, roster);
        ByzCoinRPC bc = new ByzCoinRPC(roster, adminDarc, BLOCK_INTERVAL);
        assertTrue(bc.checkLiveness());
        listBlocks(bc);
        bcStr = bc.getGenesisBlock().getId().toString();
    }

    @Test
    void Step2() throws CothorityException {
        SkipblockId bcid = new SkipblockId(Hex.parseHexBinary(bcStr));
        ByzCoinRPC bc = ByzCoinRPC.fromByzCoin(roster, bcid);
        assertTrue(bc.checkLiveness());
        listBlocks(bc);
    }

    void listBlocks(ByzCoinRPC bc) throws CothorityException {
        SkipchainRPC sc = bc.getSkipchain();
        SkipBlock sb = sc.getSkipblock(bc.getGenesisBlock().getId());
        for (;;){
            logger.info(Hex.printHexBinary(sb.getHash()));
            List<ForwardLink> fls = sb.getForwardLinks();
            if ( fls.size() == 0){
                break;
            }
            sb = sc.getSkipblock(fls.get(0).getTo());
        }
    }
}
