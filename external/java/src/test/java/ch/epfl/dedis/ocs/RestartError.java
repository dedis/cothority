package ch.epfl.dedis.ocs;

import ch.epfl.dedis.integration.TestServerController;
import ch.epfl.dedis.integration.TestServerInit;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.darc.SignerEd25519;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.proto.SkipchainProto;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class RestartError {
    Roster roster;
    static String ocsStr = "";
    private final static Logger logger = LoggerFactory.getLogger(RestartError.class);

    @BeforeEach
    void init() {
        TestServerController testServerController = TestServerInit.getInstance();
        roster = testServerController.getRoster();
    }

    @Test
    void Step1() throws CothorityException {
        Signer admin = new SignerEd25519();
        Darc adminDarc = new Darc(admin, null, null);
        OnchainSecretsRPC ocs = new OnchainSecretsRPC(roster, adminDarc);
        ocs.verify();
        listBlocks(ocs);
        ocsStr = ocs.ocsID.toString();
    }

    @Test
    void Step2() throws CothorityException {
        SkipblockId ocsid = new SkipblockId(Hex.parseHexBinary(ocsStr));
        OnchainSecretsRPC ocs = new OnchainSecretsRPC(roster, ocsid);
        ocs.verify();
        listBlocks(ocs);
    }

    void listBlocks(OnchainSecretsRPC ocs) throws CothorityException {
        SkipchainProto.SkipBlock sb = ocs.getSkipblock(ocs.ocsID);
        for (;;){
            logger.info(Hex.printHexBinary(sb.getHash().toByteArray()));
            if (sb.getForwardCount() == 0){
                break;
            }
            SkipblockId next = new SkipblockId(sb.getForward(0).getTo().toByteArray());
            sb = ocs.getSkipblock(next);
        }
    }
}
