package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.LocalRosters;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.DarcIdentity;
import ch.epfl.dedis.lib.darc.Ed25519Identity;
import ch.epfl.dedis.lib.darc.Ed25519Signer;
import ch.epfl.dedis.lib.darc.IdentityFactory;
import ch.epfl.dedis.lib.darc.SignaturePath;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.ocs.Document;
import ch.epfl.dedis.ocs.WriteRequest;
import ch.epfl.dedis.ocs.WriteRequestId;
import org.junit.jupiter.api.Test;

import javax.xml.bind.DatatypeConverter;
import java.net.URISyntaxException;
import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.assertNotNull;

public class GrantAccessTest {
    static final String SUPERADMIN_SCALAR = "AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D";
    static final String PUBLISHER_SCALAR ="69DBF32C1F19445487D3B0FF92919BD9F01D5B2314492D82FE74DE37EA0CF635";
    static final String CONSUMER_SCALAR = "3DA69196EBDCF765FF9DA6E65AB811EB19EA56D246AD4022A423AC84D1B36A02";

    @Test
    void attemptToGrantAccessBeforeCreationDirectlyToKey() throws Exception {
        // given
        // setup skipchain
        SkipblockId genesis = createSkipChainForTest();
        OnchainSecrets ocs = connectToExistingSkipchain(genesis);
        DarcId publisherId = createPublisher(ocs);

        Ed25519Signer publisherSigner = new Ed25519Signer(DatatypeConverter.parseHexBinary(PUBLISHER_SCALAR));

        //when
        DarcIdentity publisherIdentity = new DarcIdentity(publisherId);
        Darc documentDarc = new Darc(publisherIdentity, Arrays.asList(publisherIdentity), "document darc".getBytes());
        ocs.updateDarc(documentDarc);
        ocs.addIdentityToDarc(documentDarc,
                new Ed25519Identity(new Ed25519Signer(DatatypeConverter.parseHexBinary(CONSUMER_SCALAR))),
                publisherSigner, SignaturePath.USER);

        Document doc = new Document("ala ma kota".getBytes(), 16, documentDarc, "extra data".getBytes());

        WriteRequest writeId = ocs.publishDocument(doc, publisherSigner);
        WriteRequestId documentId = new WriteRequestId(writeId.id.getId()); // recreate object to ensure separation

        //then
        Document document = ocs.getDocument(documentId, new Ed25519Signer(DatatypeConverter.parseHexBinary(CONSUMER_SCALAR)));
        assertNotNull(document.getDataEncrypted());
    }

    @Test
    void attemptToGrantAccessToExistingDocumentDirectlyToKey() throws Exception {
        // given
        // setup skipchain
        SkipblockId genesis = createSkipChainForTest();
        OnchainSecrets ocs = connectToExistingSkipchain(genesis);
        DarcId publisherId = createPublisher(ocs);

        Ed25519Signer publisherSigner = new Ed25519Signer(DatatypeConverter.parseHexBinary(PUBLISHER_SCALAR));

        DarcIdentity publisherIdentity = new DarcIdentity(publisherId);
        Darc documentDarc = new Darc(publisherIdentity, Arrays.asList(publisherIdentity), "docuemnt darc".getBytes());


        Document doc = new Document("ala ma kota".getBytes(), 16, documentDarc, "extra data".getBytes());

        WriteRequest writeId = ocs.publishDocument(doc, publisherSigner);
        WriteRequestId documentId = new WriteRequestId(writeId.id.getId()); // recreate object to ensure separation

        //when
        ocs.addIdentityToDarc(documentDarc,
                new Ed25519Identity(new Ed25519Signer(DatatypeConverter.parseHexBinary(CONSUMER_SCALAR))),
                publisherSigner, SignaturePath.USER);

        //then
        Document document = ocs.getDocument(documentId, new Ed25519Signer(DatatypeConverter.parseHexBinary(CONSUMER_SCALAR)));
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

        Ed25519Signer publisherSigner = new Ed25519Signer(DatatypeConverter.parseHexBinary(PUBLISHER_SCALAR));

        DarcIdentity publisherIdentity = new DarcIdentity(publisherId);
        Darc documentDarc = new Darc(publisherIdentity, Arrays.asList(publisherIdentity), "docuemnt darc".getBytes());


        Document doc = new Document("ala ma kota".getBytes(), 16, documentDarc, "extra data".getBytes());

        WriteRequest writeId = ocs.publishDocument(doc, publisherSigner);
        WriteRequestId documentId = new WriteRequestId(writeId.id.getId()); // recreate object to ensure separation

        //when
        ocs.addIdentityToDarc(documentDarc, new DarcIdentity(consumerId), publisherSigner, SignaturePath.USER);

        //then
        Document document = ocs.getDocument(documentId, new Ed25519Signer(DatatypeConverter.parseHexBinary(CONSUMER_SCALAR)));
        assertNotNull(document.getDataEncrypted());
    }

    private DarcId createConsumer(OnchainSecrets ocs) throws Exception {
        Darc user = createUser(ocs, new Ed25519Identity(new Ed25519Signer(DatatypeConverter.parseHexBinary(CONSUMER_SCALAR))));
        return new DarcId(user.getId().getId());
    }

    private OnchainSecrets connectToExistingSkipchain(SkipblockId genesis) throws Exception {
        OcsFactory ocsFactory = new OcsFactory();
        ocsFactory.addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1);
        ocsFactory.setGenesis(genesis);
        return ocsFactory.createConnection();
    }

    private DarcId createPublisher(OnchainSecrets ocs) throws Exception {
        Darc user = createUser(ocs, new Ed25519Identity(new Ed25519Signer(DatatypeConverter.parseHexBinary(PUBLISHER_SCALAR))));
        grantSystemWriteAccess(ocs, user);
        return new DarcId(user.getId().getId()); // copy to be sure that it is not the same object
    }

    private SkipblockId createSkipChainForTest() throws URISyntaxException, CothorityCommunicationException, CothorityCryptoException {
        return new OcsFactory()
                .addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1)
                .addConode(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2)
                .addConode(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3)
                .initialiseNewSkipchain(new Ed25519Signer(
                        DatatypeConverter.parseHexBinary(SUPERADMIN_SCALAR)));
    }

    private static Darc createUser(OnchainSecrets ocs, Ed25519Identity user) throws Exception {
        Darc userDarc = new Darc(user, Arrays.asList(user), "user".getBytes());
        ocs.updateDarc(userDarc);
        return userDarc;
    }

    private static void grantSystemWriteAccess(OnchainSecrets ocs, Darc userDarc) throws Exception {
        Ed25519Signer admin = new Ed25519Signer(DatatypeConverter.parseHexBinary(SUPERADMIN_SCALAR));
        ocs.addIdentityToDarc(ocs.getAdminDarc(), IdentityFactory.New(userDarc), admin, SignaturePath.USER);
        ocs.addIdentityToDarc(ocs.getAdminDarc(), IdentityFactory.New(userDarc), admin, SignaturePath.OWNER);
    }
}
