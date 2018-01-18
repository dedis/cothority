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
import ch.epfl.dedis.ocs.Document;
import ch.epfl.dedis.ocs.WriteRequest;
import ch.epfl.dedis.ocs.WriteRequestId;
import ch.epfl.dedis.proto.OCSProto;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInstance;

import javax.xml.bind.DatatypeConverter;
import java.util.Arrays;
import java.util.Collections;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
public class GetPathTest {
    static final String SUPERADMIN_SCALAR = "AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D";
    static final String PUBLISHER_SCALAR = "69DBF32C1F19445487D3B0FF92919BD9F01D5B2314492D82FE74DE37EA0CF635";
    static final String CONSUMER_SCALAR = "3DA69196EBDCF765FF9DA6E65AB811EB19EA56D246AD4022A423AC84D1B36A02";
    private OnchainSecrets ocs;
    private DarcId publisherId;
    private DarcId consumerId;
    private DarcId readersGroupId;

    @BeforeAll
    void setupBlokchainAndUsers() throws Exception {
        SkipblockId genesis = createSkipChainForTest();
        ocs = connectToExistingSkipchain(genesis);
        publisherId = createPublisher(ocs);
        consumerId = createConsumer(ocs);
        readersGroupId = createUserGroup(ocs, consumerId);
    }

    @Test
    void checkAccessUsingKeyWithProperAccess() throws Exception {
        // given
        WriteRequestId documentId = publishDocumentAndGrantAccessToGroup();

        Ed25519Signer consumer = new Ed25519Signer(DatatypeConverter.parseHexBinary(CONSUMER_SCALAR));

        OCSProto.Write document = ocs.getWrite(documentId);
        Darc documentDarc = new Darc(document.getReader());

        // when
        SignaturePath path = ocs.getDarcPath(documentDarc.getId(), IdentityFactory.New(consumer), SignaturePath.USER);

        //then
        assertNotNull(path);
    }


    @Test
    void checkAccessUsingKeyWithoutAccess() throws Exception {
        // given
        WriteRequestId documentId = publishDocumentAndGrantAccessToGroup();

        Ed25519Signer userWithoutAccess = new Ed25519Signer(); // random key

        OCSProto.Write document = ocs.getWrite(documentId);
        Darc documentDarc = new Darc(document.getReader());

        // when
        CothorityCommunicationException exception = assertThrows(CothorityCommunicationException.class, () ->
                ocs.getDarcPath(documentDarc.getId(), IdentityFactory.New(userWithoutAccess), SignaturePath.USER));

        //then
        assertEquals("didn't find a path to the given identity", exception.getMessage()); // consider it as 'permission deny'
    }

    @Test
    void checkAccessUsingUserId() throws Exception {
        // given
        WriteRequestId documentId = publishDocumentAndGrantAccessToGroup();

        DarcIdentity consumerIdentity = new DarcIdentity(consumerId);

        OCSProto.Write document = ocs.getWrite(documentId);
        Darc documentDarc = new Darc(document.getReader());

        // when
        SignaturePath path = ocs.getDarcPath(documentDarc.getId(), consumerIdentity, SignaturePath.USER);

        //then
        assertNotNull(path);
    }

    @Test
    void checkAccessUsingGroupId() throws Exception {
        // given
        WriteRequestId documentId = publishDocumentAndGrantAccessToGroup();

        DarcIdentity groupIdentity = new DarcIdentity(readersGroupId);

        OCSProto.Write document = ocs.getWrite(documentId);
        Darc documentDarc = new Darc(document.getReader());

        // when
        SignaturePath path = ocs.getDarcPath(documentDarc.getId(), groupIdentity, SignaturePath.USER);

        //then
        assertNotNull(path);
    }

    private WriteRequestId publishDocumentAndGrantAccessToGroup() throws Exception {
        WriteRequestId documentId;
        Ed25519Signer publisherSigner = new Ed25519Signer(DatatypeConverter.parseHexBinary(PUBLISHER_SCALAR));
        documentId = publishTestDocument(publisherSigner, publisherId, readersGroupId);
        return documentId;
    }

    private WriteRequestId publishTestDocument(Ed25519Signer publisherSigner, DarcId publisherDarcId, DarcId consumerId) throws Exception {
        DarcIdentity publisherIdentity = new DarcIdentity(publisherDarcId);
        DarcIdentity consumerIdentity = new DarcIdentity(consumerId);

        Darc documentDarc = new Darc(publisherIdentity, Arrays.asList(publisherIdentity), "document darc".getBytes());
        ocs.updateDarc(documentDarc);
        ocs.addIdentityToDarc(documentDarc, consumerIdentity, publisherSigner, SignaturePath.USER);

        Document doc = new Document("ala ma kota".getBytes(), 16, documentDarc, "extra data".getBytes());

        WriteRequest writeId = ocs.publishDocument(doc, publisherSigner);
        return new WriteRequestId(writeId.id.getId()); // recreate object to ensure separation
    }

    private DarcId createUserGroup(OnchainSecrets ocs, DarcId... members) throws Exception {
        Ed25519Signer admin = new Ed25519Signer(DatatypeConverter.parseHexBinary(SUPERADMIN_SCALAR));

        Darc groupDarc = new Darc(admin, Collections.emptyList(), "group".getBytes());
        for (DarcId id : members) {
            groupDarc.addUser(new DarcIdentity(id));
        }
        ocs.updateDarc(groupDarc);
        return groupDarc.getId();
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

    private SkipblockId createSkipChainForTest() throws Exception {
        return new OcsFactory()
                .addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1)
                .addConode(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2)
                .addConode(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3)
                .addConode(LocalRosters.CONODE_4, LocalRosters.CONODE_PUB_4)
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
