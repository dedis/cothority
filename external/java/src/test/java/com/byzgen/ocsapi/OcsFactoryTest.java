package com.byzgen.ocsapi;

import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.ocs.LocalRosters;
import ch.epfl.dedis.ocs.OnchainSecrets;
import com.byzgen.ocsapi.OcsFactory;
import org.junit.jupiter.api.Test;

import java.net.URI;
import java.net.URISyntaxException;

import static org.hamcrest.CoreMatchers.containsString;
import static org.hamcrest.CoreMatchers.not;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.hamcrest.text.IsEmptyString.isEmptyOrNullString;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

class OcsFactoryTest {
    public static final String SAMPLE_GENESIS_ID = "jdnQTgJwQOaBXVi1zMyx+hPfdxGY1S8+A1yr3/w0VRo=";
    public static final String GENESIS_ID_WITH_SPACE = "jdnQ TgJwQOaBXVi1zMyx+hPfdxGY1S8+A1yr3/w0VRo=";
    public static final String GENESIS_ID_TOO_SHORT = "jdnQTgJwQOaBXVi1zMyx";

    public static final String PUBLIC_KEY_WITH_SPACE = "base64WithSpace TvMRQrO1PAw2pVjA1hDMQQi7Tss=";
    public static final String CONODE_ADDRESS_INCORRECT = "http://127.0.0.1:7002";

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
        assertThat(exception.getMessage(), containsString("Illegal base64 character"));
    }

    @Test
    public void shouldFailWhenServersAreNotSpecified() {
        OcsFactory ocsFactory = new OcsFactory();
        Throwable exception = assertThrows(IllegalStateException.class, () -> {
            ocsFactory.setGenesis(SAMPLE_GENESIS_ID);
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
    public void shouldFailWhenGenesisIsMalformed() {
        OcsFactory ocsFactory = new OcsFactory();
        Throwable exception = assertThrows(IllegalArgumentException.class, () -> {
            ocsFactory.setGenesis(GENESIS_ID_WITH_SPACE);
        });
        assertThat(exception.getMessage(), containsString("Illegal base64 character"));
    }

    @Test
    public void shouldFailWhenGenesisIsTooShort() {
        OcsFactory ocsFactory = new OcsFactory();
        Throwable exception = assertThrows(IllegalArgumentException.class, () -> {
            ocsFactory.setGenesis(GENESIS_ID_TOO_SHORT);
        });
        assertThat(exception.getMessage(), containsString("Genesis value is too short"));
    }

//    @Test
    public void shouldInitialiseSkipChain() throws Exception {
        OcsFactory ocsFactory = new OcsFactory();

        ocsFactory.addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1);
        ocsFactory.addConode(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2);
        ocsFactory.addConode(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3);

        ocsFactory.initialiseNewChain();
        OnchainSecrets conection = ocsFactory.createConnection();

        String genesis = conection.getGenesis();
        assertThat(genesis, not(isEmptyOrNullString()));
    }

//    @Test
    public void shouldCreateConnectionToExistingChain() throws Exception {
        // given
        final String genesis = createSkipChainForTest();

        OcsFactory ocsFactory = new OcsFactory();
        ocsFactory.addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1);
        ocsFactory.setGenesis(genesis);

        // when
        OnchainSecrets conection = ocsFactory.createConnection();

        // then
        assertNotNull(conection);
    }

    private String createSkipChainForTest() throws URISyntaxException, CothorityCommunicationException {
        return new OcsFactory().addConode(LocalRosters.CONODE_1, LocalRosters.CONODE_PUB_1)
                .addConode(LocalRosters.CONODE_2, LocalRosters.CONODE_PUB_2)
                .addConode(LocalRosters.CONODE_3, LocalRosters.CONODE_PUB_3)
                .initialiseNewChain()
                .createConnection()
                .getGenesis();
    }
}