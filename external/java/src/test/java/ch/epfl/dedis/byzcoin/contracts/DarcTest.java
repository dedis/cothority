package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.SignerCounters;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.Argument;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.lib.crypto.SignerX509ECTest;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;

import static ch.epfl.dedis.byzcoin.ByzCoinRPCTest.BLOCK_INTERVAL;
import static org.junit.jupiter.api.Assertions.*;

class DarcTest {
    static ByzCoinRPC bc;

    static Signer admin;
    static Darc genesisDarc;

    private final static Logger logger = LoggerFactory.getLogger(DarcTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws Exception {
        testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());

        bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, BLOCK_INTERVAL);
        if (!bc.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }
    }

    @Test
    void updateDarc() throws Exception {
        // Get the counter for the admin
        SignerCounters counters = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        DarcInstance dc = DarcInstance.fromByzCoin(bc, genesisDarc);
        logger.info("DC is: {}", dc.getId());
        logger.info("genesisDarc is: {}", genesisDarc.getId());
        Darc newDarc = genesisDarc.partialCopy();
        newDarc.setRule("spawn:darc", "all".getBytes());

        Instruction instr = dc.evolveDarcInstruction(newDarc, counters.head()+1);
        logger.info("DC is: {}", dc.getId());

        ClientTransaction ctx = new ClientTransaction(Arrays.asList(instr));
        ctx.signWith(Collections.singletonList(admin));
        bc.sendTransactionAndWait(ctx, 10);

        dc.update();
        logger.info("darc-version is: {}", dc.getDarc().getVersion());
        assertEquals(newDarc.getVersion(), dc.getDarc().getVersion());
    }

    @Test
    void keycardSignature() throws Exception {
        // Get the counter for the admin
        SignerCounters counters = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        // Evolve to give kcsigner the evolve permission
        SignerX509EC kcsigner = new SignerX509ECTest();
        SignerX509EC kcsigner2 = new SignerX509ECTest();
        Darc adminDarc2 = genesisDarc.partialCopy();
        adminDarc2.setRule(Darc.RuleEvolve, kcsigner.getIdentity().toString().getBytes());
        DarcInstance di = DarcInstance.fromByzCoin(bc, genesisDarc);
        di.evolveDarcAndWait(adminDarc2, admin, counters.head()+1, 10);
        di.update();
        assertEquals(1, di.getDarc().getVersion());
        assertTrue(new String(di.getDarc().getExpression(Darc.RuleEvolve)).contains(kcsigner.getIdentity().toString()));

        final Darc adminDarc3 = adminDarc2.partialCopy();
        assertThrows(Exception.class, () -> {
                    logger.info("Trying to evolve darc with wrong signer: " + kcsigner2.getIdentity().toString());
                    adminDarc3.setRule(Darc.RuleEvolve, kcsigner2.getIdentity().toString().getBytes());
                    di.evolveDarcAndWait(adminDarc3, kcsigner2, 1L, 10);
                }
        );
        di.update();
        assertEquals(1, di.getDarc().getVersion());

        // Evolve to give kcsigner2 the permission
        final Darc adminDarc3bis = adminDarc2.partialCopy();
        adminDarc3bis.setRule(Darc.RuleEvolve, kcsigner2.getIdentity().toString().getBytes());
        logger.info("Updating darc with new signer: " + kcsigner.getIdentity().toString());
        di.evolveDarcAndWait(adminDarc3bis, kcsigner, 1L, 10);
        di.update();
        assertEquals(2, di.getDarc().getVersion());
    }

    @Test
    void spawnDarc() throws Exception {
        // Get the counter for the admin
        SignerCounters counters = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        DarcInstance dc = DarcInstance.fromByzCoin(bc, genesisDarc);
        Darc darc2 = genesisDarc.partialCopy();
        darc2.setRule("spawn:darc", admin.getIdentity().toString().getBytes());
        dc.evolveDarcAndWait(darc2, admin, counters.head()+1, 10);

        List<Identity> id = Arrays.asList(admin.getIdentity());
        Darc newDarc = new Darc(id, id, "new darc".getBytes());

        Proof p = dc.spawnInstanceAndWait("darc", admin, counters.head()+2,
                Argument.NewList("darc", newDarc.toProto().toByteArray()), 10);
        assertTrue(p.matches());

        logger.info("creating DarcInstance");
        DarcInstance dc2 = DarcInstance.fromByzCoin(bc, newDarc);
        logger.info("ids: {} - {}", dc2.getDarc().getId(), newDarc.getId());
        logger.info("ids: {} - {}", dc2.getDarc().getBaseId(), newDarc.getBaseId());
        logger.info("darcs:\n{}\n{}", dc2.getDarc(), newDarc);
        assertTrue(dc2.getDarc().getId().equals(newDarc.getId()));
    }


    @Test
    void addAccountToByzcoin() throws CothorityException {
        Darc admin3Darc = bc.getGenesisDarc();
        Signer reader = new SignerEd25519();
        admin3Darc.addIdentity(Darc.RuleSignature, reader.getIdentity(), Rules.OR);
        assertNotNull(admin3Darc);
    }

    @Test
    void addDarcs() throws CothorityException {
        // Get the counter for the admin
        SignerCounters counters = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        DarcInstance gi = bc.getGenesisDarcInstance();
        List<DarcId> ids = new ArrayList<>();
        // Add 50 darcs without waiting - so the requests will be batched together in multiple blocks
        int n = 50;
        for (int i = 0; i < n; i++) {
            logger.info("Adding darc {}", i);
            Signer newSigner = new SignerEd25519();
            Darc newDarc = new Darc(Arrays.asList(newSigner.getIdentity()), Arrays.asList(newSigner.getIdentity()), "new darc".getBytes());
            gi.spawnDarcAndWait(newDarc, admin, counters.head()+1+i, 0);
            ids.add(newDarc.getBaseId());
        }

        // Add a last one and wait for it, hoping the leader does not rearrange them.
        Signer newSigner = new SignerEd25519();
        Darc newDarc = new Darc(Arrays.asList(newSigner.getIdentity()), Arrays.asList(newSigner.getIdentity()), "new darc".getBytes());
        gi.spawnDarcAndWait(newDarc, admin, counters.head()+1+n, 10);
        ids.add(newDarc.getBaseId());

        // Verify all the darcs have been correctly written by getting their proofs from ByzCoin.
        ids.forEach(id -> {
            try {
                Proof p = bc.getProof(new InstanceId(id.getId()));
                assertTrue(p.matches());
                assertEquals(DarcInstance.ContractId, p.getContractID());
            } catch (CothorityException e) {
                fail("Got exception when fetching darc: " + e.getMessage());
            }
        });
    }
}
