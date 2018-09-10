package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.ocs.Document;
import ch.epfl.dedis.ocs.WriteRequest;
import ch.epfl.dedis.ocs.WriteRequestId;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.assertNotNull;

public class GrantAccessTest {
    static final String SUPERADMIN_SCALAR = "AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D";
    private final SignerX509EC consumerSigner = new TestSignerX509EC();
    private final SignerX509EC publisherSigner = new TestSignerX509EC();
    private final SignerX509EC consumerPublicPart = new TestLimitedSignerX509EC(consumerSigner);
    private TestServerController testServerController;

    @BeforeEach
    void initConodes() {
        testServerController = TestServerInit.getInstance();
    }

    @Test
    void attemptToGrantAccessBeforeCreationDirectlyToKey() throws Exception {
        // given
        // setup skipchain
        SkipblockId genesis = createSkipChainForTest();
        OnchainSecrets ocs = connectToExistingSkipchain(genesis);
        DarcId publisherId = createPublisher(ocs);

        //when
        IdentityDarc publisherIdentity = new IdentityDarc(publisherId);
        Darc documentDarc = new Darc(publisherIdentity, Arrays.asList(publisherIdentity), "document darc".getBytes());
        ocs.updateDarc(documentDarc);
        ocs.addIdentityToDarc(documentDarc, new IdentityX509EC(consumerPublicPart),
                publisherSigner, SignaturePath.USER);

        Document doc = new Document("ala ma kota".getBytes(), 16, documentDarc, "extra data".getBytes());

        WriteRequest writeId = ocs.publishDocument(doc, publisherSigner);
        WriteRequestId documentId = new WriteRequestId(writeId.id.getId()); // recreate object to ensure separation

        //then
        Document document = ocs.getDocumentEphemeral(documentId, consumerSigner);
        assertNotNull(document.getDataEncrypted());
    }

    @Test
    void attemptToGrantAccessToExistingDocumentDirectlyToKey() throws Exception {
        // given
        // setup skipchain
        SkipblockId genesis = createSkipChainForTest();
        OnchainSecrets ocs = connectToExistingSkipchain(genesis);
        DarcId publisherId = createPublisher(ocs);

        IdentityDarc publisherIdentity = new IdentityDarc(publisherId);
        Darc documentDarc = new Darc(publisherIdentity, Arrays.asList(publisherIdentity), "document darc".getBytes());


        Document doc = new Document("ala ma kota".getBytes(), 16, documentDarc, "extra data".getBytes());

        WriteRequest writeId = ocs.publishDocument(doc, publisherSigner);
        WriteRequestId documentId = new WriteRequestId(writeId.id.getId()); // recreate object to ensure separation

        //when
        Identity identityX509EC = new IdentityX509EC(consumerPublicPart);
        ocs.addIdentityToDarc(documentDarc, identityX509EC, publisherSigner, SignaturePath.USER);

        //then
        Document document = ocs.getDocumentEphemeral(documentId, consumerSigner);
        assertNotNull(document.getDataEncrypted());
    }

    @Test
    void attemptToGrantAccessToExistingDocumentToOtherDarc() throws Exception {
        // given
        // setup skipchain
        SkipblockId genesis = createSkipChainForTest();
        OnchainSecrets ocs = connectToExistingSkipchain(genesis);
        DarcId publisherId = createPublisher(ocs);
        DarcId consumerId = createConsumer(ocs);

        IdentityDarc publisherIdentity = new IdentityDarc(publisherId);
        Darc documentDarc = new Darc(publisherIdentity, Arrays.asList(publisherIdentity), "docuemnt darc".getBytes());


        Document doc = new Document("ala ma kota".getBytes(), 16, documentDarc, "extra data".getBytes());

        WriteRequest writeId = ocs.publishDocument(doc, publisherSigner);
        WriteRequestId documentId = new WriteRequestId(writeId.id.getId()); // recreate object to ensure separation

        //when
        ocs.addIdentityToDarc(documentDarc, new IdentityDarc(consumerId), publisherSigner, SignaturePath.USER);

        //then
        Document document = ocs.getDocumentEphemeral(documentId, consumerSigner);
        assertNotNull(document.getDataEncrypted());
    }

    private DarcId createConsumer(OnchainSecrets ocs) throws Exception {
        Darc user = createUser(ocs, consumerSigner.getIdentity());
        return new DarcId(user.getId().getId());
    }

    private OnchainSecrets connectToExistingSkipchain(SkipblockId genesis) throws Exception {
        OcsFactory ocsFactory = new OcsFactory();
        ocsFactory.addConode(testServerController.getMasterConode());
        ocsFactory.setGenesis(genesis);
        return ocsFactory.createConnection();
    }

    private DarcId createPublisher(OnchainSecrets ocs) throws Exception {
        Darc user = createUser(ocs, new IdentityX509EC(publisherSigner));
        grantSystemWriteAccess(ocs, user);
        return new DarcId(user.getId().getId()); // copy to be sure that it is not the same object
    }

    private SkipblockId createSkipChainForTest() throws CothorityCommunicationException {
        return new OcsFactory()
                .addConodes(testServerController.getConodes())
                .initialiseNewSkipchain(new SignerEd25519(
                        Hex.parseHexBinary(SUPERADMIN_SCALAR)));
    }

    private static Darc createUser(OnchainSecrets ocs, Identity user) throws Exception {
        Darc userDarc = new Darc(user, Arrays.asList(user), "user".getBytes());
        ocs.updateDarc(userDarc);
        return userDarc;
    }

    private static void grantSystemWriteAccess(OnchainSecrets ocs, Darc userDarc) throws Exception {
        SignerEd25519 admin = new SignerEd25519(Hex.parseHexBinary(SUPERADMIN_SCALAR));
        ocs.addIdentityToDarc(ocs.getAdminDarc(), IdentityFactory.New(userDarc), admin, SignaturePath.USER);
        ocs.addIdentityToDarc(ocs.getAdminDarc(), IdentityFactory.New(userDarc), admin, SignaturePath.OWNER);
    }

    private class TestLimitedSignerX509EC extends TestSignerX509EC {
        public TestLimitedSignerX509EC(SignerX509EC consumerKeys) {
            super(consumerKeys.getPublicKey(), null);
        }

        @Override
        public byte[] sign(byte[] msg) throws SignRequestRejectedException {
            throw new SignRequestRejectedException("It is not acceptable to sign message when access is granted to the user", null);
        }
    }
}
