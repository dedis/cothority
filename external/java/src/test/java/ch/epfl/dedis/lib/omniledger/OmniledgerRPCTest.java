package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.contracts.DarcInstance;
import ch.epfl.dedis.lib.omniledger.contracts.ValueInstance;
import ch.epfl.dedis.lib.omniledger.darc.*;
import com.google.protobuf.ByteString;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.*;

public class OmniledgerRPCTest {
    static OmniledgerRPC ol;

    static Signer admin;
    static Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(OmniledgerRPCTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        Rules rules = Darc.initRules(Arrays.asList(admin.getIdentity()),
                Arrays.asList(admin.getIdentity()));
        genesisDarc = new Darc(rules, "genesis".getBytes());

        ol = new OmniledgerRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(100, MILLIS));
        if (!ol.checkLiveness()){
            throw new CothorityCommunicationException("liveness check failed");
        }
    }

    @Test
    void ping() throws Exception{
        assertTrue(ol.checkLiveness());
    }

    @Test
    void updateDarc() throws Exception {
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        logger.info("DC is: {}", dc.getId());
        logger.info("genesisDarc is: {}", genesisDarc.getId());
        Darc newDarc = genesisDarc.copy();
        newDarc.setRule("spawn:darc", "all".getBytes());
        Instruction instr = dc.evolveDarcInstruction(newDarc, admin, 0, 1);
        logger.info("DC is: {}", dc.getId());
        ol.sendTransactionAndWait(new ClientTransaction(Arrays.asList(instr)), 10);

        dc.update();
        logger.info("darc-version is: {}", dc.getDarc().getVersion());
        assertEquals(newDarc.getVersion(), dc.getDarc().getVersion());
    }

    @Test
    void spawnDarc() throws Exception{
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        Darc darc2 = genesisDarc.copy();
        darc2.setRule("spawn:darc", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin);

        List<Identity> id = Arrays.asList(admin.getIdentity());
        Darc newDarc = new Darc(id, id, "new darc".getBytes());

        Proof p = dc.spawnContractAndWait("darc", admin,
                Argument.NewList("darc", newDarc.toProto().toByteArray()), 10);
        assertTrue(p.matches());

        logger.info("creating DarcInstance");
        DarcInstance dc2 = new DarcInstance(ol, newDarc);
        logger.info("ids: {} - {}", dc2.getDarc().getId(), newDarc.getId());
        logger.info("ids: {} - {}", dc2.getDarc().getBaseId(), newDarc.getBaseId());
        logger.info("darcs:\n{}\n{}", dc2.getDarc(), newDarc);
        assertTrue(dc2.getDarc().getId().equals(newDarc.getId()));
    }

    @Test
    void spawnValue() throws Exception{
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        Darc darc2 = genesisDarc.copy();
        darc2.setRule("spawn:value", admin.getIdentity().toString().getBytes());
        darc2.setRule("invoke:update", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin);

        byte[] myvalue = "314159".getBytes();
        Proof p = dc.spawnContractAndWait("value", admin, Argument.NewList("value", myvalue), 10);
        assertTrue(p.matches());

        ValueInstance vi = new ValueInstance(ol, p);
        assertArrayEquals(vi.getValue(), myvalue);
        myvalue = "27".getBytes();
        vi.evolveValueAndWait(myvalue, admin);
        assertArrayEquals(vi.getValue(), myvalue);
    }

    @Test
    void failToSpawnValue() throws Exception{
        // In this test we send through a transaction we know is going to fail
        // in order to verify that the txid shows up in the refused transactions
        // list in the next block. We then use spawnContractAndWait on one we know is
        // going to succeed in order to sync the test to the creation of the new
        // block.
        DarcInstance dc = new DarcInstance(ol, genesisDarc);
        Darc darc2 = genesisDarc.copy();
        darc2.setRule("spawn:value", admin.getIdentity().toString().getBytes());
        darc2.setRule("invoke:update", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin);

        // Send thru a tx with the wrong signer so it fails.
        Signer user = new SignerEd25519();
        ClientTransactionId txid = dc.spawnContract("value", user, Argument.NewList("value", "314159".getBytes()));

        // And send through a valid tx too, that we can wait for, so we know a block just got processed.
        Proof p = dc.spawnContractAndWait("value", admin, Argument.NewList("value", "314159".getBytes()), 10);
        assertTrue(p.matches());

        // Now that we know the latest block (it was returned to us in the proof p), we check it for the expected
        // failed tx. If we don't find it, we walk backwards one and look. We need to check back because OL could
        // decide to try one block with our failed tx, commit it, and then try another block with the success tx.

        OmniBlock ob = new OmniBlock(p);
        List<TxResult> txr = ob.getTxResults();

        // If there are extra tx's we were not expecting, then abort.
        assertTrue(txr.size() <= 2);

        // both tx ended up in one block
        if (txr.size() == 2) {
            // Index: 0: genesis, 1: darc evolution, 2: this block with both failed and succeeded in it.
            assertEquals(2, p.getLatest().getIndex());

            ClientTransactionId ref;
            if (!txr.get(0).isAccepted()) {
                ref = txr.get(0).getClientTransaction().getId();
            } else {
                ref = txr.get(1).getClientTransaction().getId();
            }
            assertTrue(ref.equals(txid));
            return;
        }

        // This one must have been the accepted tx. Confirm that.
        assertTrue(txr.get(0).isAccepted());

        // Look back one block for the expected failed tx.
        assertEquals(1, p.getLatest().getProto().getBacklinksCount());
        SkipblockId back = new SkipblockId(p.getLatest().getProto().getBacklinks(0));
        SkipBlock b = ol.getSkipchain().getSkipblock(back);
        ob = new OmniBlock(b);
        txr = ob.getTxResults();
        assertEquals(1, txr.size());
        assertFalse(txr.get(0).isAccepted());

        ClientTransactionId ref = txr.get(0).getClientTransaction().getId();
        assertTrue(txr.get(0).getClientTransaction().getId().equals(txid));
    }

    /**
     * We only give the client the roster and the genesis ID. It should be able to find the configuration, latest block
     * and the genesis darc.
     */
    @Test
    void reconnect() throws Exception {
        OmniledgerRPC ol2 = new OmniledgerRPC(ol.getRoster(), ol.getGenesis().getSkipchainId());
        assertEquals(ol.getConfig(), ol2.getConfig());
        assertEquals(ol.getLatestOmniBlock().getTimestampNano(), ol2.getLatestOmniBlock().getTimestampNano());
        assertEquals(ol.getGenesisDarc().getBaseId(), ol2.getGenesisDarc().getBaseId());
    }
}
