package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.calypso.LTSId;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.calypso.CalypsoRPC;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.net.URI;

import static org.hamcrest.CoreMatchers.containsString;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

class CalypsoFactoryTest {
    public static final String SAMPLE_GENESIS_ID = "8dd9d04e027040e6815d58b5ccccb1fa13df771198d52f3e035cabdffc34551a";
    public static final String PUBLIC_KEY_WITH_SPACE = "hex with spaces TvMRQrO1PAw2pVjA1hDMQQi7Tss=";
    public static final String CONODE_ADDRESS_INCORRECT = "http://localhost:7002";
    public static final String SAMPLE_CONODE_URI = "tcp://remote.host.name:7044";
    public static final String SAMPLE_CONODE_PUB = "402552116B5056CC6B989BAE9A8DFD8BF0C1A2714FB820F0472C096AB5D148D8";

    private TestServerController testServerController;

    @BeforeEach
    void initConodes() {
        testServerController = TestServerInit.getInstance();
    }

    @Test
    public void shouldFailWhenServersAddressIsNotCorrect() {
        CalypsoFactory calypsoFactory = new CalypsoFactory();
        Throwable exception = assertThrows(IllegalArgumentException.class, () -> {
            calypsoFactory.addConode(new URI(CONODE_ADDRESS_INCORRECT), SAMPLE_CONODE_PUB);
        });
        assertThat(exception.getMessage(), containsString("address must be in tcp format"));
    }

    @Test
    public void shouldFailWhenPublicAddressIsNotCorrect() {
        CalypsoFactory calypsoFactory = new CalypsoFactory();
        Throwable exception = assertThrows(IllegalArgumentException.class, () -> {
            calypsoFactory.addConode(new URI(SAMPLE_CONODE_URI), PUBLIC_KEY_WITH_SPACE);
        });
        assertThat(exception.getMessage(), containsString("contains illegal character for hexBinary"));
    }

    @Test
    public void shouldFailWhenServersAreNotSpecified() {
        CalypsoFactory calypsoFactory = new CalypsoFactory();
        Throwable exception = assertThrows(IllegalStateException.class, () -> {
            calypsoFactory.setGenesis(new SkipblockId(Hex.parseHexBinary(SAMPLE_GENESIS_ID)));
            calypsoFactory.setLTSId(new LTSId(new byte[32]));
            calypsoFactory.createConnection();
        });
        assertThat(exception.getMessage(), containsString("No cothority server"));
    }

    @Test
    public void shouldFailWhenGenesisIsNotSpecified() {
        CalypsoFactory calypsoFactory = new CalypsoFactory();
        Throwable exception = assertThrows(IllegalStateException.class, () -> {
            calypsoFactory.addConode(new URI(SAMPLE_CONODE_URI), SAMPLE_CONODE_PUB);
            calypsoFactory.createConnection();
        });
        assertThat(exception.getMessage(), containsString("No genesis specified"));
    }

    @Test
    public void shouldInitialiseSkipChain() throws Exception {
        // given
        CalypsoFactory calypsoFactory = new CalypsoFactory();
        calypsoFactory.addConodes(testServerController.getConodes());

        // when
        SkipblockId genesis = calypsoFactory.initialiseNewCalypso(
                new SignerEd25519(Hex.parseHexBinary("AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D"))).getGenesisBlock().getSkipchainId();

        // then
        assertNotNull(genesis);
    }

    @Test
    public void shouldCreateConnectionToExistingChain() throws Exception {
        // given
        final CalypsoRPC crpc = createCalypsoForTest();

        CalypsoFactory calypsoFactory = new CalypsoFactory();
        calypsoFactory.addConode(testServerController.getMasterConode());
        calypsoFactory.setGenesis(crpc.getGenesisBlock().getSkipchainId());
        calypsoFactory.setLTSId(crpc.getLTSId());

        // when
        CalypsoRPC conection = calypsoFactory.createConnection();

        // then
        assertNotNull(conection);
    }

    private CalypsoRPC createCalypsoForTest() throws CothorityException {
        return new CalypsoFactory()
                .addConodes(testServerController.getConodes())
                .initialiseNewCalypso(new SignerEd25519(
                        Hex.parseHexBinary("AEE42B6A924BDFBB6DAEF8B252258D2FDF70AFD31852368AF55549E1DF8FC80D")));
    }
}