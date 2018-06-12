package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class OmniLedgerTest {

    private OmniLedger omni;
    private TestServerController testInstanceController;

    @BeforeEach
    void setUp() {
        List<Signer> signers =  new ArrayList<>();
        signers.add(new SignerEd25519());
        testInstanceController = TestServerInit.getInstance();
        this.omni = new OmniLedger(testInstanceController.getRoster(), );
    }

    @Test
    void addTransaction() {
    }

    @Test
    void getProof() {
    }

    @Test
    void saveConfig() {
    }
}