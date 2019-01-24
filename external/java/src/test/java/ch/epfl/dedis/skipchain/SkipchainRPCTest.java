package ch.epfl.dedis.skipchain;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.network.Roster;
import ch.epfl.dedis.lib.network.ServerIdentity;
import ch.epfl.dedis.lib.proto.SkipchainProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

import static ch.epfl.dedis.byzcoin.ByzCoinRPCTest.BLOCK_INTERVAL;
import static org.junit.jupiter.api.Assertions.*;

class SkipchainRPCTest {
    private SkipchainRPC sc;
    private SkipblockId genesisId;
    private Roster fullRoster;

    @BeforeEach
    void initAll() throws Exception {
        TestServerController testInstanceController = TestServerInit.getInstance();
        Signer admin = new SignerEd25519();
        Darc genesisDarc = ByzCoinRPC.makeGenesisDarc(admin, testInstanceController.getRoster());
        ByzCoinRPC bc = new ByzCoinRPC(testInstanceController.getRoster(), genesisDarc, BLOCK_INTERVAL);
        if (!bc.checkLiveness()) {
            throw new CothorityCommunicationException("liveness check failed");
        }
        sc = bc.getSkipchain();
        genesisId = bc.getGenesisBlock().getId();
        fullRoster = testInstanceController.getRoster();
    }

    @Test
    void checkStatus() {
        assertTrue(sc.checkStatus());
    }

    @Test
    void getSkipblock() throws Exception {
        assertEquals(sc.getSkipblock(genesisId).getId(), genesisId);
    }

    /**
     * This test uses a different skipchain because we customise the genesis block.
     */
    @Test
    void getLatestSkipblock() throws Exception {
        List<ServerIdentity> nodes = fullRoster.getNodes();
        int conodes = nodes.size();
        int sbCount = conodes - 1;
        List<SkipBlock> sbs = new ArrayList<>();

        // Initialise the genesis block, it's not the one from the initialisation function
        SkipBlock genesisSB = this.makeGenesisRosterArgs(new Roster(nodes.subList(0, 2)), null, new ArrayList<>(), 2, 3);
        sbs.add(genesisSB);

        // Initialized skipchain.
        for (int i = 1; i < sbCount; i++) {
            Roster roster = new Roster(nodes.subList(i, i+2));
            SkipchainProto.SkipBlock newSB = newSkipBlock(roster);
            SkipchainProto.StoreSkipBlockReply reply = this.storeSkipBlock(roster, sbs.get(i-1).getHash(), newSB);
            sbs.add(new SkipBlock(reply.getLatest()));
        }

        // Wait for blocks to be stored
        Thread.sleep(1000);

        // Verify the results
        for (int i = 0; i < sbCount; i++) {
            SkipchainRPC sc = new SkipchainRPC(sbs.get(i).getRoster(), genesisSB.getId());
            List<SkipBlock> sbc = sc.getUpdateChain(sbs.get(i).getId());
            assertTrue(sbc.size() > 0, "Empty update-chain");
            assertEquals(sbc.get(0), sbs.get(i), "First hash is not from our SkipBlock");
            assertEquals(sbc.get(sbc.size() - 1), sbs.get(sbCount-1), "Last Hash is not equal to last SkipBlock for");

            for (int up = 0; up < sbc.size(); up++) {
                SkipBlock sb1 = sbc.get(0);
                assertTrue(sb1.verifyForwardSignatures());
                if (up < sbc.size() - 1) {
                    SkipBlock sb2 = sbc.get(up+1);
                    int h1 = sb1.getHeight();
                    int h2 = sb2.getHeight();
                    int height = h1;
                    if (h2 < height) {
                        height = h2;
                    }
                    assertEquals(sb1.getForwardLinks().get(height-1).getTo(), sb2.getId(),
                            String.format("Forward-pointer[%d/%d] of update %d %s is different from hash in %d %s",
                                    height-1, sb1.getForwardLinks().size(), up, sb1.getForwardLinks().get(height-1).getTo(), up+1, sb2.getId()));
                }
            }
        }
    }

    private SkipBlock makeGenesisRosterArgs(Roster roster, SkipblockId parent, List<byte[]> verifierIDs, int base, int maxHeight) throws CothorityException {
        SkipchainProto.SkipBlock.Builder b = SkipchainProto.SkipBlock.newBuilder();
        b.setRoster(roster.toProto());
        b.setMaxHeight(maxHeight);
        b.setBaseHeight(base);
        if (parent != null) {
            b.setParent(parent.toProto());
        }
        b.addAllVerifiers(verifierIDs.stream().map(ByteString::copyFrom).collect(Collectors.toList()));

        // set stuff to their defaults
        b.setIndex(0);
        b.setHeight(0);
        b.setGenesis(ByteString.copyFrom(new byte[]{}));
        b.setData(ByteString.copyFrom(new byte[]{}));
        b.setHash(ByteString.copyFrom(new byte[]{}));

        SkipchainProto.StoreSkipBlockReply reply = storeSkipBlock(sc.getRoster(), new byte[]{}, b.build());
        return new SkipBlock(reply.getLatest());
    }

    private SkipchainProto.SkipBlock newSkipBlock(Roster roster) {
        return SkipchainProto.SkipBlock.newBuilder()
                .setIndex(0)
                .setHeight(0)
                .setMaxHeight(0)
                .setBaseHeight(0)
                .setGenesis(ByteString.copyFrom(new byte[]{}))
                .setData(ByteString.copyFrom(new byte[]{}))
                .setRoster(roster.toProto())
                .setHash(ByteString.copyFrom(new byte[]{}))
                .build();
    }

    private SkipchainProto.StoreSkipBlockReply storeSkipBlock(Roster roster, byte[] targetSkipChainID, SkipchainProto.SkipBlock newBlock) throws CothorityCommunicationException, CothorityCryptoException {
        try {
            SkipchainProto.StoreSkipBlock request =
                    SkipchainProto.StoreSkipBlock.newBuilder()
                            .setTargetSkipChainID(ByteString.copyFrom(targetSkipChainID))
                            .setNewBlock(newBlock)
                            .build();
            ByteString msg = roster.sendMessage("Skipchain/StoreSkipBlock", request);
            return SkipchainProto.StoreSkipBlockReply.parseFrom(msg);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }
}