package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.SignerCounters;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.byzcoin.Block;
import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.Argument;
import ch.epfl.dedis.byzcoin.transaction.ClientTransactionId;
import ch.epfl.dedis.byzcoin.transaction.TxResult;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.eventlog.EventLogInstance;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.Collections;
import java.util.List;

import static java.time.temporal.ChronoUnit.MILLIS;
import static org.junit.jupiter.api.Assertions.*;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ValueTest {
    private static ByzCoinRPC bc;
    private static EventLogInstance el;
    private static Signer admin;
    private static Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(ValueTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());

        bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, Duration.of(500, MILLIS));
        if (!bc.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }
    }


    @Test
    void spawnValue() throws Exception {

        DarcInstance dc = DarcInstance.fromByzCoin(bc, genesisDarc);

        // Get the counter for the admin
        SignerCounters adminCtrs = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        Darc darc2 = genesisDarc.copyRulesAndVersion();
        darc2.setRule("spawn:value", admin.getIdentity().toString().getBytes());
        darc2.setRule("invoke:update", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin, adminCtrs.head()+1, 10);

        byte[] myvalue = "314159".getBytes();
        Proof p = dc.spawnInstanceAndWait("value", admin, adminCtrs.head()+2, Argument.NewList("value", myvalue), 10);
        assertTrue(p.matches());

        ValueInstance vi = ValueInstance.fromByzcoin(bc, p);
        assertArrayEquals(vi.getValue(), myvalue);
        myvalue = "27".getBytes();
        vi.evolveValueAndWait(myvalue, admin, adminCtrs.head()+3, 10);
        assertArrayEquals(vi.getValue(), myvalue);

        // this part is a regression test for
        // https://github.com/dedis/cothority/issues/1527
        Block ob = new Block(p);
        ob.getAcceptedClientTransactions()
                .forEach(clientTransaction -> clientTransaction.getInstructions().
                        forEach(instr -> processInstr(instr)));
    }

    void processInstr(Instruction instr) {
        try {
            instr.deriveId("");
        } catch (CothorityCryptoException e) {
            throw new RuntimeException(e);
        }
    }

    @Test
    void failToSpawnValue() throws Exception {
        // In this test we send through a transaction we know is going to fail
        // in order to verify that the txid shows up in the refused transactions
        // list in the next block. We then use spawnInstanceAndWait on one we know is
        // going to succeed in order to sync the test to the creation of the new
        // block.
        DarcInstance dc = DarcInstance.fromByzCoin(bc, genesisDarc);

        // Get the counter for the admin
        SignerCounters adminCtrs = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        Darc darc2 = genesisDarc.copyRulesAndVersion();
        darc2.setRule("spawn:value", admin.getIdentity().toString().getBytes());
        darc2.setRule("invoke:update", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin, adminCtrs.head()+1, 10);

        // Send through a tx with the wrong signer so it fails.
        Signer user = new SignerEd25519();
        ClientTransactionId txid = dc.spawnInstance("value", user, 1L, Argument.NewList("value", "314159".getBytes()));

        // And send through a valid tx too, that we can wait for, so we know a block just got processed.
        Proof p = dc.spawnInstanceAndWait("value", admin, adminCtrs.head()+2, Argument.NewList("value", "314159".getBytes()), 10);
        assertTrue(p.matches());

        // Now that we know the latest block (it was returned to us in the proof p), we check it for the expected
        // failed tx. If we don't find it, we walk backwards one and look. We need to check back because BC could
        // decide to try one block with our failed tx, commit it, and then try another block with the success tx.

        Block ob = new Block(p);
        List<TxResult> txr = ob.getTxResults();

        // If there are extra tx's we were not expecting, then abort.
        assertTrue(txr.size() <= 2);

        // both tx ended up in one block
        if (txr.size() == 2) {
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
        SkipBlock b = bc.getSkipchain().getSkipblock(back);
        ob = new Block(b);
        txr = ob.getTxResults();
        assertEquals(1, txr.size());
        assertFalse(txr.get(0).isAccepted());

        ClientTransactionId ref = txr.get(0).getClientTransaction().getId();
        assertTrue(txr.get(0).getClientTransaction().getId().equals(txid));
    }


}
