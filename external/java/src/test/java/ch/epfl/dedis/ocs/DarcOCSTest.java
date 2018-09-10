package ch.epfl.dedis.ocs;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

public class DarcOCSTest {
    static OnchainSecrets ocs;
    static Signer admin;
    static Signer publisher;
    static Darc adminDarc;
    private final static Logger logger = LoggerFactory.getLogger(OnchainSecretsRPCTest.class);
    private TestServerController testInstanceController;

    @BeforeEach
    void initAll() throws CothorityException {
        admin = new SignerEd25519();
        publisher = new SignerEd25519();

        adminDarc = new Darc(admin, Arrays.asList(publisher), null);

        testInstanceController = TestServerInit.getInstance();

        try {
            logger.info("Admin darc: " + adminDarc.getId().toString());
            ocs = new OnchainSecrets(testInstanceController.getRoster(), adminDarc);
        } catch (Exception e){
            logger.error("Couldn't start skipchain - perhaps you need to run the following commands:");
            logger.error("cd $(go env GOPATH)/src/github.com/dedis/onchain-secrets/conode");
            logger.error("./run_conode.sh local 4 2");
        }
    }

    @Test
    void keycardSignature() throws Exception{
        SignerX509EC kcsigner = new TestSignerX509EC();
        SignerX509EC kcsigner2 = new TestSignerX509EC();
        Darc adminDarc2;
        adminDarc2 = ocs.addIdentityToDarc(adminDarc, kcsigner.getIdentity(), admin, SignaturePath.OWNER);

        Darc adminDarc3;
        assertThrows(Exception.class, ()->
                ocs.addIdentityToDarc(adminDarc2, kcsigner2.getIdentity(), kcsigner2, SignaturePath.OWNER)
        );
        logger.info(adminDarc2.owners.get(1).toProto().toString());
        logger.info(kcsigner2.getIdentity().toProto().toString());
        adminDarc3 = ocs.addIdentityToDarc(adminDarc2, kcsigner2.getIdentity(), kcsigner, SignaturePath.OWNER);
        assertNotNull(adminDarc3);
    }
}
