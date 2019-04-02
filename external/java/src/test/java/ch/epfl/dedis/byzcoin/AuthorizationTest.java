package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.byzcoin.contracts.SecureDarcInstance;
import ch.epfl.dedis.byzgen.CalypsoFactory;
import ch.epfl.dedis.calypso.CalypsoRPC;
import ch.epfl.dedis.calypso.Document;
import ch.epfl.dedis.calypso.LTSId;
import ch.epfl.dedis.calypso.WriteInstance;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.*;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInstance;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class AuthorizationTest {
    static final String SUPERADMIN_SCALAR = "AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D";
    static final String PUBLISHER_SCALAR = "69DBF32C1F19445487D3B0FF92919BD9F01D5B2314492D82FE74DE37EA0CF635";
    static final String CONSUMER_SCALAR = "3DA69196EBDCF765FF9DA6E65AB811EB19EA56D246AD4022A423AC84D1B36A02";
    private CalypsoRPC calypso;
    private DarcId publisherId;
    private DarcId consumerId;
    private DarcId readersGroupId;
    private TestServerController testServerController;
    private Signer admin;
    private WriteInstance writeInstance;

    private final Logger logger = LoggerFactory.getLogger(AuthorizationTest.class);

    @BeforeAll
    void setupBlokchainAndUsers() throws Exception {
        admin = new SignerEd25519(Hex.parseHexBinary(SUPERADMIN_SCALAR));

        testServerController = TestServerInit.getInstance();
        CalypsoRPC crpc = createSkipChainForTest();
        calypso = connectToExistingSkipchain(crpc.getGenesisBlock().getSkipchainId(), crpc.getLTSId());
        logger.info("creating publisher");
        publisherId = createPublisher(calypso);
        logger.info("publisherId is: {}", Hex.printHexBinary(publisherId.getId()));
        logger.info("creating consumer");
        consumerId = createConsumer(calypso);
        logger.info("consumerId is: {}", Hex.printHexBinary(consumerId.getId()));
        readersGroupId = createUserGroup(calypso, consumerId);
        logger.info("creating document");
        writeInstance = publishDocumentAndGrantAccessToGroup();
    }

    @Test
    void checkAccessUsingKeyWithProperAccess() throws Exception {
        // given
        assertNotNull(writeInstance);

        // Check that consumer has access
        SignerEd25519 consumer = new SignerEd25519(Hex.parseHexBinary(CONSUMER_SCALAR));

        List<String> auths = calypso.checkAuthorization(writeInstance.getInstance().getDarcBaseID(), Arrays.asList(consumer.getIdentity()));
        assertEquals(1, auths.size());
    }

    @Test
    void checkAccessUsingKeyWithoutAccess() throws Exception {
        // given a writeInstanceId, but not the instance iteslf
        InstanceId writeInstanceId = writeInstance.getInstance().getId();
        // and a random key
        SignerEd25519 userWithoutAccess = new SignerEd25519();

        // Fetch the instance from ByzCoin
        WriteInstance wi = WriteInstance.fromCalypso(calypso, writeInstanceId);

        // when
        List<String> auths = calypso.checkAuthorization(wi.getDarcId(),
                Arrays.asList(IdentityFactory.New(userWithoutAccess)));

        // then
        assertEquals(0, auths.size());
    }

    @Test
    void checkAccessUsingUserId() throws Exception {
        // given a darc pointing to a consumer
        IdentityDarc consumerIdentity = new IdentityDarc(consumerId);

        // when
        List<String> auths = calypso.checkAuthorization(writeInstance.getDarcId(),
                Arrays.asList(consumerIdentity));

        // then
        assertTrue(auths.contains("spawn:calypsoRead"));
    }

    @Test
    void checkAccessUsingGroupId() throws Exception {
        // given a darc pointing to a group
        IdentityDarc groupIdentity = new IdentityDarc(readersGroupId);

        // when
        List<String> auths = calypso.checkAuthorization(writeInstance.getDarcId(),
                Arrays.asList(groupIdentity));

        // then
        logger.info("Authentications are: {}", auths);
        assertTrue(auths.contains("spawn:calypsoRead"));
    }

    private WriteInstance publishDocumentAndGrantAccessToGroup() throws Exception {
        WriteInstance documentId;
        SignerEd25519 publisherSigner = new SignerEd25519(Hex.parseHexBinary(PUBLISHER_SCALAR));
        documentId = publishTestDocument(publisherSigner, 1L, publisherId, readersGroupId);
        return documentId;
    }

    private WriteInstance publishTestDocument(SignerEd25519 publisherSigner, Long publisherSignerCtr, DarcId publisherDarcId, DarcId consumerId) throws Exception {
        // Get the counter for the admin
        SignerCounters adminCtrs = calypso.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        IdentityDarc publisherIdentity = new IdentityDarc(publisherDarcId);
        IdentityDarc consumerIdentity = new IdentityDarc(consumerId);

        Darc documentDarc = new Darc(Arrays.asList(publisherIdentity), null, "document darc".getBytes());
        documentDarc.addIdentity("spawn:calypsoWrite", publisherIdentity, Rules.OR);
        documentDarc.addIdentity("spawn:calypsoRead", consumerIdentity, Rules.OR);
        calypso.getGenesisDarcInstance().spawnDarcAndWait(documentDarc, admin, adminCtrs.head()+1, 10);

        Document doc = new Document("ala ma kota".getBytes(), "extra data".getBytes(), documentDarc.getBaseId());
        return doc.spawnWrite(calypso, documentDarc.getBaseId(), publisherSigner, publisherSignerCtr);
    }

    private DarcId createUserGroup(CalypsoRPC ocs, DarcId... members) throws Exception {
        SignerEd25519 admin = new SignerEd25519(Hex.parseHexBinary(SUPERADMIN_SCALAR));

        // Get the counter for the admin
        SignerCounters adminCtrs = calypso.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        Darc groupDarc = new Darc(Arrays.asList(admin.getIdentity()), null, "group".getBytes());
        for (DarcId id : members) {
            groupDarc.addIdentity(Darc.RuleSignature, new IdentityDarc(id), Rules.OR);
        }

        ocs.getGenesisDarcInstance().spawnDarcAndWait(groupDarc, admin, adminCtrs.head()+1, 10);
        return groupDarc.getId();
    }

    private DarcId createConsumer(CalypsoRPC ocs) throws Exception {
        SecureDarcInstance user = createUser(ocs, new IdentityEd25519(new SignerEd25519(Hex.parseHexBinary(CONSUMER_SCALAR))));
        return new DarcId(user.getId().getId());
    }

    private CalypsoRPC connectToExistingSkipchain(SkipblockId genesis, LTSId ltsId) throws Exception {
        CalypsoFactory calypsoFactory = new CalypsoFactory();
        calypsoFactory.addConode(testServerController.getMasterConode());
        calypsoFactory.setGenesis(genesis);
        calypsoFactory.setLTSId(ltsId);
        return calypsoFactory.createConnection();
    }

    private DarcId createPublisher(CalypsoRPC ocs) throws Exception {
        SecureDarcInstance user = createUser(ocs, new IdentityEd25519(new SignerEd25519(Hex.parseHexBinary(PUBLISHER_SCALAR))));
        grantSystemWriteAccess(ocs, user.getDarc());
        return new DarcId(user.getId().getId()); // copy to be sure that it is not the same object
    }

    private CalypsoRPC createSkipChainForTest() throws Exception {
        return new CalypsoFactory()
                .addConodes(testServerController.getConodes())
                .initialiseNewCalypso(new SignerEd25519(
                        Hex.parseHexBinary(SUPERADMIN_SCALAR)));
    }

    private SecureDarcInstance createUser(CalypsoRPC ocs, IdentityEd25519 user) throws Exception {
        SignerEd25519 admin = new SignerEd25519(Hex.parseHexBinary(SUPERADMIN_SCALAR));
        SignerCounters adminCtrs = calypso.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        Darc userDarc = new Darc(Arrays.asList(user), Arrays.asList(user), "user".getBytes());
        return ocs.getGenesisDarcInstance().spawnDarcAndWait(userDarc, admin, adminCtrs.head()+1, 10);
    }

    private void grantSystemWriteAccess(CalypsoRPC ocs, Darc userDarc) throws Exception {
        SignerEd25519 admin = new SignerEd25519(Hex.parseHexBinary(SUPERADMIN_SCALAR));
        SignerCounters adminCtrs = calypso.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));

        Darc newGenesis = ocs.getGenesisDarc().partialCopy();
        newGenesis.addIdentity(Darc.RuleSignature, IdentityFactory.New(userDarc), Rules.OR);
        newGenesis.addIdentity(Darc.RuleEvolve, IdentityFactory.New(userDarc), Rules.OR);

        SecureDarcInstance di = SecureDarcInstance.fromByzCoin(ocs, ocs.getGenesisDarc().getBaseId());
        di.evolveDarcAndWait(newGenesis, admin, adminCtrs.head()+1, 10);
    }
}
