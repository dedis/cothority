package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Ed25519Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.LocalRosters;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.ocs.OnchainSecretsRPC;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import javax.xml.bind.DatatypeConverter;
import java.net.URI;
import java.net.URISyntaxException;

import static org.hamcrest.CoreMatchers.containsString;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

class OcsFactoryTest {
    public static final String SAMPLE_GENESIS_ID = "8dd9d04e027040e6815d58b5ccccb1fa13df771198d52f3e035cabdffc34551a";
    public static final String PUBLIC_KEY_WITH_SPACE = "hex with spaces TvMRQrO1PAw2pVjA1hDMQQi7Tss=";
    public static final String CONODE_ADDRESS_INCORRECT = "http://127.0.0.1:7002";

    @BeforeEach
    void initConodes() {
        TestServerInit.getInstance();
    }

    @Test
    public void shouldFailWhenServersAddressIsNotCorrect() {
        OcsFactory ocsFactory = new OcsFactory();
        Throwable exception = assertThrows(IllegalArgumentException.class, () -> {
            ocsFactory.addConode(new URI(CONODE_ADDRESS_INCORRECT), LocalRosters.CONODE_PUB_1);
        });
        assertThat(exception.getMessage(), containsString("address must be in tcp format"));
    }

    @Test
    public void shouldFailWhenPublicAddressIsNotCorrect() {
        OcsFactory ocsFactory = new OcsFactory();
        Throwable exception = assertThrows(IllegalArgumentException.class, () -> {
            ocsFactory.addConode(LocalRosters.CONODE_1, PUBLIC_KEY_WITH_SPACE);
        });
        assertThat(exception.getMessage(), containsString("contains illegal character for hexBinary"));
    }

    @Test
    public void shouldFailWhenServersAreNotSpecified() {
        OcsFactory ocsFactory = new OcsFactory();
        Throwable exception = assertThrows(IllegalStateException.class, () -> {
            ocsFactory.setGenesis(new SkipblockId(DatatypeConverter.parseHexBinary(SAMPLE_GENESIS_ID)));
            ocsFactory.createConnection();
        });
        assertThat(exception.getMessage(), containsString("No cothority server"));
    }

    @Test
    public void shouldFailWhenGenesisIsNotSpecified() {
        OcsFactory ocsFactory = new OcsFactory();
        Throwable exception = assertThrows(IllegalStateException.class, () -> {
            ocsFactory.addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1);
            ocsFactory.createConnection();
        });
        assertThat(exception.getMessage(), containsString("No genesis specified"));
    }

    @Test
    public void shouldInitialiseSkipChain() throws Exception {
        // given
        OcsFactory ocsFactory = new OcsFactory();
        ocsFactory.addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1);
        ocsFactory.addConode(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2);
        ocsFactory.addConode(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3);
        ocsFactory.addConode(LocalRosters.CONODE_4, LocalRosters.CONODE_PUB_4);

        // when
        SkipblockId genesis = ocsFactory.initialiseNewSkipchain(
                new Ed25519Signer(DatatypeConverter.parseHexBinary("AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D")));

        // then
        assertNotNull(genesis);
    }

    @Test
    public void shouldCreateConnectionToExistingChain() throws Exception {
        // given
        final SkipblockId genesis = createSkipChainForTest();

        OcsFactory ocsFactory = new OcsFactory();
        ocsFactory.addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1);
        ocsFactory.setGenesis(genesis);

        // when
        OnchainSecretsRPC conection = ocsFactory.createConnection();

        // then
        assertNotNull(conection);
    }

    private SkipblockId createSkipChainForTest() throws URISyntaxException, CothorityCommunicationException, CothorityCryptoException {
        return new OcsFactory()
                .addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1)
                .addConode(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2)
                .addConode(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3)
                .addConode(LocalRosters.CONODE_4, LocalRosters.CONODE_PUB_4)
                .initialiseNewSkipchain(new Ed25519Signer(
                        DatatypeConverter.parseHexBinary("AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D")));
    }
}