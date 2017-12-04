package ch.epfl.dedis.byzgen;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.ocs.WriteRequestId;
import sun.reflect.generics.reflectiveObjects.NotImplementedException;

public class OnchainSecrets extends ch.epfl.dedis.ocs.OnchainSecrets {
    /**
     * Creates a new OnchainSecrets class that attaches to an existing skipchain.
     *
     * @param roster
     * @param ocsID
     */
    public OnchainSecrets(Roster roster, SkipblockId ocsID) throws CothorityCommunicationException, CothorityCryptoException {
        super(roster, ocsID);
    }

    /**
     * Creates a new OnchainSecrets class and creates a new skipchain.
     *
     * @param roster
     * @param admin
     */
    public OnchainSecrets(Roster roster, Darc admin) throws CothorityCommunicationException, CothorityCryptoException {
        super(roster, admin);
    }

    /**
     * Searches for the most up-to-date readers-darc of the given write
     * request.
     *
     * @param id
     * @return
     */
    public Darc getLatestReaders(WriteRequestId id) {
        throw new NotImplementedException();
    }
}

