package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.byzcoin.contracts.SecureDarcInstance;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import ch.epfl.dedis.lib.proto.TrieProto;
import com.google.protobuf.ByteString;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.stream.Collectors;

import static ch.epfl.dedis.byzcoin.ByzCoinRPCTest.BLOCK_INTERVAL;
import static org.junit.jupiter.api.Assertions.*;

class ProofTest {
    private ByzCoinRPC bc;
    private Signer admin;

    @BeforeEach
    void initAll() throws Exception {
        TestServerController testInstanceController = TestServerInit.getInstance();
        admin = new SignerEd25519();
        Darc genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());

        bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, BLOCK_INTERVAL);
        if (!bc.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }
    }

    @Test
    void test() throws Exception {
        // create a bunch of blocks
        SignerCounters counters = bc.getSignerCounters(Collections.singletonList(admin.getIdentity().toString()));
        SecureDarcInstance gi = bc.getGenesisDarcInstance();
        List<DarcId> ids = new ArrayList<>();
        int n = 4;
        for (int i = 0; i < n; i++) {
            Signer newSigner = new SignerEd25519();
            Darc newDarc = new Darc(Collections.singletonList(newSigner.getIdentity()),
                    Collections.singletonList(newSigner.getIdentity()),
                    ("new darc" + i).getBytes());
            // we wait so that there are more blocks
            gi.spawnDarcAndWait(newDarc, admin, counters.head()+1+i, 10);
            ids.add(newDarc.getBaseId());
        }

        // get the proof for all the darcs, it should pass
        for (DarcId id : ids) {
            Proof p = bc.getProof(new InstanceId(id.getId()));
            assertTrue(p.exists(id.getId()));
        }

        // get proof for something that doesn't exist, it should return false but should not throw
        InstanceId badId = new InstanceId("aaaaaaaabbbbbbbbccccccccdddddddd".getBytes());
        assertFalse(bc.getProof(badId).exists(badId.getId()));

        // if the skipchain ID is wrong, it should fail
        InstanceId iid = new InstanceId(ids.get(0).getId());
        Proof p = bc.getProof(new InstanceId(ids.get(0).getId()));
        assertThrows(CothorityCryptoException.class, () ->
                new Proof(p.toProto(), new SkipblockId(new byte[32]), iid));

        // useful variables for constructing a bad proof
        TrieProto.Proof inclusionProof = p.toProto().getInclusionproof();
        TrieProto.LeafNode leaf = inclusionProof.getLeaf();
        assertTrue(inclusionProof.getInteriorsCount() > 1);
        TrieProto.InteriorNode interior1 = inclusionProof.getInteriors(1);
        List<Boolean> prefixList = leaf.getPrefixList();
        assertTrue(prefixList.size() > 0);

        // take one proof and modify the leaf, existence check throw
        ByzCoinProto.Proof badProtoProof = p.toProto().toBuilder()
                .setInclusionproof(inclusionProof.toBuilder()
                        .setLeaf(leaf.toBuilder()
                                .setKey(ByteString.copyFrom("abcdefg".getBytes()))))
                .build();
        assertThrows(CothorityCryptoException.class,
                () -> new Proof(badProtoProof, bc.getGenesisBlock().getId(), iid).exists(iid.getId()));

        // no interior nodes
        ByzCoinProto.Proof badProtoProof2 = p.toProto().toBuilder()
                .setInclusionproof(inclusionProof.toBuilder()
                        .clearInteriors())
                .build();
        assertThrows(CothorityCryptoException.class,
                () -> new Proof(badProtoProof2, bc.getGenesisBlock().getId(), iid).exists(iid.getId()));

        // take one proof and modify an intermediate, it should fail
        ByzCoinProto.Proof badProtoProof3 = p.toProto().toBuilder()
                .setInclusionproof(inclusionProof.toBuilder()
                        .setInteriors(1, interior1.toBuilder()
                                .setLeft(ByteString.copyFrom("gauche".getBytes()))))
                .build();
        assertThrows(CothorityCryptoException.class,
                () -> new Proof(badProtoProof3, bc.getGenesisBlock().getId(), iid).exists(iid.getId()));

        // wrong prefix (we invert the prefix)
        ByzCoinProto.Proof badProtoProof4 = p.toProto().toBuilder()
                .setInclusionproof(inclusionProof.toBuilder()
                        .setLeaf(leaf.toBuilder()
                                .clearPrefix()
                                .addAllPrefix(prefixList.stream().map(x -> !x).collect(Collectors.toList()))))
                .build();
        assertThrows(CothorityCryptoException.class,
                () -> new Proof(badProtoProof4, bc.getGenesisBlock().getId(), iid).exists(iid.getId()));

        // wrong block hash
        SkipchainProto.SkipBlock badBlock = p.getLatest().getProto().toBuilder()
                .setBaseHeight(123)
                .build();
        ByzCoinProto.Proof badProtoProof5 = p.toProto().toBuilder()
                .setLatest(badBlock)
                .build();
        assertThrows(CothorityCryptoException.class,
                () -> new Proof(badProtoProof5, bc.getGenesisBlock().getId(), iid).exists(iid.getId()));
    }
}