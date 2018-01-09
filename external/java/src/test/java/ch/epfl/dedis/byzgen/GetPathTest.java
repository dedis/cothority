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
        WriteRequestId documentId = publishDocuentAndGrantAccessToGroup();

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
        WriteRequestId documentId = publishDocuentAndGrantAccessToGroup();

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
        WriteRequestId documentId = publishDocuentAndGrantAccessToGroup();

        DarcIdentity consumerIdentity = new DarcIdentity(consumerId);

        OCSProto.Write document = ocs.getWrite(documentId);
        Darc documentDarc = new Darc(document.getReader());

        // when
        SignaturePath path = ocs.getDarcPath(documentDarc.getId(), consumerIdentity, SignaturePath.USER);

        //then
        assertNotNull(path);
    }

    /*
During execution of this test there is java.lang.NullPointerException.
There is also 'nil pointer dereference' at server side.

  NPE:

java.lang.NullPointerException
at ch.epfl.dedis.lib.ServerIdentity.SendMessage(ServerIdentity.java:71)
at ch.epfl.dedis.lib.Roster.sendMessage(Roster.java:53)
at ch.epfl.dedis.ocs.OnchainSecretsRPC.getDarcPath(OnchainSecretsRPC.java:240)
at ch.epfl.dedis.byzgen.GetPathTest.checkAccessUsingGroupId(GetPathTest.java:112)
at sun.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
at sun.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
at sun.reflect.DelegatingMethodAccessorImpl.invoke(DelegatingMethodAccessorImpl.java:43)
at java.lang.reflect.Method.invoke(Method.java:498)
at org.junit.platform.commons.util.ReflectionUtils.invokeMethod(ReflectionUtils.java:389)
at org.junit.jupiter.engine.execution.ExecutableInvoker.invoke(ExecutableInvoker.java:115)
at org.junit.jupiter.engine.descriptor.TestMethodTestDescriptor.lambda$invokeTestMethod$6(TestMethodTestDescriptor.java:167)
at org.junit.jupiter.engine.execution.ThrowableCollector.execute(ThrowableCollector.java:40)
at org.junit.jupiter.engine.descriptor.TestMethodTestDescriptor.invokeTestMethod(TestMethodTestDescriptor.java:163)
at org.junit.jupiter.engine.descriptor.TestMethodTestDescriptor.execute(TestMethodTestDescriptor.java:110)
at org.junit.jupiter.engine.descriptor.TestMethodTestDescriptor.execute(TestMethodTestDescriptor.java:57)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.lambda$execute$3(HierarchicalTestExecutor.java:83)
at org.junit.platform.engine.support.hierarchical.SingleTestExecutor.executeSafely(SingleTestExecutor.java:66)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.execute(HierarchicalTestExecutor.java:77)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.lambda$null$2(HierarchicalTestExecutor.java:92)
at java.util.stream.ForEachOps$ForEachOp$OfRef.accept(ForEachOps.java:184)
at java.util.stream.ReferencePipeline$2$1.accept(ReferencePipeline.java:175)
at java.util.Iterator.forEachRemaining(Iterator.java:116)
at java.util.Spliterators$IteratorSpliterator.forEachRemaining(Spliterators.java:1801)
at java.util.stream.AbstractPipeline.copyInto(AbstractPipeline.java:481)
at java.util.stream.AbstractPipeline.wrapAndCopyInto(AbstractPipeline.java:471)
at java.util.stream.ForEachOps$ForEachOp.evaluateSequential(ForEachOps.java:151)
at java.util.stream.ForEachOps$ForEachOp$OfRef.evaluateSequential(ForEachOps.java:174)
at java.util.stream.AbstractPipeline.evaluate(AbstractPipeline.java:234)
at java.util.stream.ReferencePipeline.forEach(ReferencePipeline.java:418)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.lambda$execute$3(HierarchicalTestExecutor.java:92)
at org.junit.platform.engine.support.hierarchical.SingleTestExecutor.executeSafely(SingleTestExecutor.java:66)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.execute(HierarchicalTestExecutor.java:77)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.lambda$null$2(HierarchicalTestExecutor.java:92)
at java.util.stream.ForEachOps$ForEachOp$OfRef.accept(ForEachOps.java:184)
at java.util.stream.ReferencePipeline$2$1.accept(ReferencePipeline.java:175)
at java.util.Iterator.forEachRemaining(Iterator.java:116)
at java.util.Spliterators$IteratorSpliterator.forEachRemaining(Spliterators.java:1801)
at java.util.stream.AbstractPipeline.copyInto(AbstractPipeline.java:481)
at java.util.stream.AbstractPipeline.wrapAndCopyInto(AbstractPipeline.java:471)
at java.util.stream.ForEachOps$ForEachOp.evaluateSequential(ForEachOps.java:151)
at java.util.stream.ForEachOps$ForEachOp$OfRef.evaluateSequential(ForEachOps.java:174)
at java.util.stream.AbstractPipeline.evaluate(AbstractPipeline.java:234)
at java.util.stream.ReferencePipeline.forEach(ReferencePipeline.java:418)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.lambda$execute$3(HierarchicalTestExecutor.java:92)
at org.junit.platform.engine.support.hierarchical.SingleTestExecutor.executeSafely(SingleTestExecutor.java:66)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.execute(HierarchicalTestExecutor.java:77)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestExecutor.execute(HierarchicalTestExecutor.java:51)
at org.junit.platform.engine.support.hierarchical.HierarchicalTestEngine.execute(HierarchicalTestEngine.java:43)
at org.junit.platform.launcher.core.DefaultLauncher.execute(DefaultLauncher.java:170)
at org.junit.platform.launcher.core.DefaultLauncher.execute(DefaultLauncher.java:154)
at org.junit.platform.launcher.core.DefaultLauncher.execute(DefaultLauncher.java:90)
at com.intellij.junit5.JUnit5IdeaTestRunner.startRunnerWithArgs(JUnit5IdeaTestRunner.java:65)
at com.intellij.rt.execution.junit.IdeaTestRunner$Repeater.startRunnerWithArgs(IdeaTestRunner.java:47)
at com.intellij.rt.execution.junit.JUnitStarter.prepareStreamsAndStart(JUnitStarter.java:242)
at com.intellij.rt.execution.junit.JUnitStarter.main(JUnitStarter.java:70)


And server side logs are:

2018/01/09 11:02:28 http: panic serving 172.17.0.1:38704: runtime error: invalid memory address or nil pointer dereference
goroutine 4400 [running]:
net/http.(*conn).serve.func1(0xc420b534a0)
/usr/local/go/src/net/http/server.go:1721 +0xd0
panic(0x8b9580, 0xbc1990)
/usr/local/go/src/runtime/panic.go:489 +0x2cf
github.com/dedis/onchain-secrets/service.(*Service).GetDarcPath(0xc420101520, 0xc420f1bdb0, 0x410d28, 0x20, 0xc4202d56b0)
/go/src/github.com/dedis/onchain-secrets/service/service.go:199 +0x89
github.com/dedis/onchain-secrets/service.(*Service).GetDarcPath-fm(0xc420f1bdb0, 0x0, 0x0, 0x0)
/go/src/github.com/dedis/onchain-secrets/service/service.go:842 +0x34
reflect.Value.call(0x8a6a20, 0xc4200fb9d0, 0x13, 0x9485a6, 0x4, 0xc4215d19c0, 0x1, 0x1, 0x8f9660, 0xc420f1bd60, ...)
/usr/local/go/src/reflect/value.go:434 +0x91f
reflect.Value.Call(0x8a6a20, 0xc4200fb9d0, 0x13, 0xc4215d19c0, 0x1, 0x1, 0x0, 0x0, 0xc420dc29f8)
/usr/local/go/src/reflect/value.go:302 +0xa4
github.com/dedis/onet.(*ServiceProcessor).ProcessClientRequest.func1(0x8b6301, 0x8a6a20, 0xc4200fb9d0, 0xb9ae40, 0x8f9660, 0xc420dc2a80, 0xc4200fb930, 0x3, 0xc420dc2a58, 0x56e2c8, ...)
/go/src/github.com/dedis/onet/processor.go:133 +0x34d
github.com/dedis/onet.(*ServiceProcessor).ProcessClientRequest(0xc4200fb930, 0xc4214d3d24, 0xb, 0xc420d40000, 0x6c, 0x600, 0x0, 0x0, 0x0, 0x0, ...)
/go/src/github.com/dedis/onet/processor.go:141 +0xaa
github.com/dedis/onet.wsHandler.ServeHTTP(0x94c2af, 0xe, 0xb94680, 0xc420101520, 0xb94a80, 0xc420400fc0, 0xc420ce9300)
/go/src/github.com/dedis/onet/websocket.go:144 +0x454
github.com/dedis/onet.(*wsHandler).ServeHTTP(0xc4202226e0, 0xb94a80, 0xc420400fc0, 0xc420ce9300)
<autogenerated>:146 +0x86
net/http.(*ServeMux).ServeHTTP(0xc420102690, 0xb94a80, 0xc420400fc0, 0xc420ce9300)
/usr/local/go/src/net/http/server.go:2238 +0x130
net/http.serverHandler.ServeHTTP(0xc420076580, 0xb94a80, 0xc420400fc0, 0xc420ce9300)
/usr/local/go/src/net/http/server.go:2568 +0x92
net/http.(*conn).serve(0xc420b534a0, 0xb954c0, 0xc420f4dd40)
/usr/local/go/src/net/http/server.go:1825 +0x612
created by net/http.(*Server).Serve
/usr/local/go/src/net/http/server.go:2668 +0x2ce
Tue Jan  9 11:02:43 UTC 2018

 */
    @Test
    void checkAccessUsingGroupId() throws Exception {
        // given
        WriteRequestId documentId = publishDocuentAndGrantAccessToGroup();

        DarcIdentity groupIdentity = new DarcIdentity(readersGroupId);

        OCSProto.Write document = ocs.getWrite(documentId);
        Darc documentDarc = new Darc(document.getReader());

        // when
        SignaturePath path = ocs.getDarcPath(documentDarc.getId(), groupIdentity, SignaturePath.USER);

        //then
        assertNotNull(path);
    }

    private WriteRequestId publishDocuentAndGrantAccessToGroup() throws Exception {
        WriteRequestId documentId;Ed25519Signer publisherSigner = new Ed25519Signer(DatatypeConverter.parseHexBinary(PUBLISHER_SCALAR));
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
