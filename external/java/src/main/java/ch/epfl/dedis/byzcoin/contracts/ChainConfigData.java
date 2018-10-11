package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.exception.CothorityPermissionException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.net.URISyntaxException;
import java.time.Duration;
import java.util.HashSet;
import java.util.Set;

public class ChainConfigData {
    public static final int blocksizeMin = 16000;
    public static final int blocksizeMax = 8000000;
    private ByzCoinProto.ChainConfig.Builder config;

    private static final Logger logger = LoggerFactory.getLogger(ChainConfigData.class);

    /**
     * Construct a config given the instance of the existing config.
     *
     * @param inst the instance
     * @throws CothorityNotFoundException if the read request in the instance is corrupt
     */
    public ChainConfigData(Instance inst) throws CothorityNotFoundException {
        if (!inst.getContractId().equals(ChainConfigInstance.ContractId)) {
            throw new CothorityNotFoundException("wrong contract type in instance");
        }
        try {
            config = ByzCoinProto.ChainConfig.parseFrom(inst.getData()).toBuilder();
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityNotFoundException("couldn't decode the data: " + e.getMessage());
        }
    }

    /**
     * Construct a config given the protobuf-representation of the config.
     *
     * @param config the existing config
     * @throws CothorityNotFoundException if the read request in the instance is corrupt
     */
    public ChainConfigData(ByzCoinProto.ChainConfig config) {
        this.config = config.toBuilder();
    }

    /**
     * Construct a copy of the object.
     *
     * @param config the old config
     * @throws CothorityNotFoundException if the read request in the instance is corrupt
     */
    public ChainConfigData(ChainConfigData config) {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        try {
            config.toProto().writeTo(out);
            out.close();
            this.config = ByzCoinProto.ChainConfig.parseFrom(out.toByteArray()).toBuilder();
        } catch (IOException e) {
            throw new RuntimeException("Couldn't write to output stream: " + e.getMessage());
        }
    }

    /**
     * Sets a new roster for ByzCoin. This current version is very simplified as it only allows
     * a new permutation of the roster and not to change the set of rosters. The new permutation
     * makes sense if you want to redefine the leader that is responsible for announcing new
     * blocks. The first node in the Roster will be the leader.
     *
     * @param newRoster the new roster to use
     * @throws CothorityPermissionException    if the new roster is not correctly set up.
     * @throws CothorityCommunicationException if the old roster contained an error.
     */
    public void setRoster(Roster newRoster) throws CothorityPermissionException, CothorityCommunicationException {
        Set<ServerIdentity> newSIs = new HashSet(newRoster.getNodes());
        Set<ServerIdentity> oldSIs;
        try {
            oldSIs = new HashSet(new Roster(config.getRoster()).getNodes());
        } catch (URISyntaxException e) {
            throw new CothorityCommunicationException("Error in stored roster:" + e.getMessage());
        }
        if (newSIs.containsAll(oldSIs) && oldSIs.containsAll(newSIs)) {
            return;
        }
        throw new CothorityPermissionException("Not allowed to change the set of rosters in this version!");
    }

    /**
     * This is once we have the possibility to change the set of rosters, which is not the case for the moment.
     *
     * @param newRoster roster to check
     * @throws CothorityException if something went bad
     */
    private void checkNewRoster(Roster newRoster) throws CothorityException {
        Set<ServerIdentity> newSIs = new HashSet(newRoster.getNodes());
        Set<ServerIdentity> oldSIs;
        try {
            oldSIs = new HashSet(new Roster(config.getRoster()).getNodes());
        } catch (URISyntaxException e) {
            throw new CothorityCommunicationException("Error in stored roster:" + e.getMessage());
        }
        if (Math.abs(newSIs.size() - oldSIs.size()) > 1) {
            throw new CothorityPermissionException("Not allowed to change size of roster by more than one");
        }
        if (newSIs.size() < 4) {
            throw new CothorityPermissionException("Not allowed to have less than 4 nodes");
        }
        if (newSIs.size() % 3 != 1) {
            logger.warn("this is a non-optimal size of the roster. It should be 3n+1.");
        }
        if (!newSIs.containsAll(oldSIs) && !oldSIs.containsAll(newSIs)) {
            newSIs.addAll(oldSIs);
            logger.info("SIs are: {}", newSIs);
            throw new CothorityPermissionException("More than one node difference");
        }
        config.setRoster(newRoster.toProto());
    }

    /**
     * Sets the interval between two blocks. You cannot chose an interval smaller than 5 seconds.
     *
     * @param newInterval the new interval - bigger or equal to 5 seconds.
     * @throws CothorityPermissionException if the chosen interval is wrong.
     */
    public void setInterval(Duration newInterval) throws CothorityPermissionException {
        logger.warn("Please restart the conodes, so that the services can pick up the new interval");
        if (newInterval.toMillis() < 5000) {
            throw new CothorityPermissionException("The interval should never be smaller than 5 seconds");
        }
        config.setBlockinterval(newInterval.toNanos());
    }

    /**
     * Sets the new maximum block size, which must be bigger than 2**14 bytes or 16kiBytes.
     *
     * @param newSize how many bytes should fit maximally into a new block
     * @throws CothorityPermissionException if the given size is wrong
     */
    public void setMaxBlockSize(int newSize) throws CothorityPermissionException {
        if (newSize < blocksizeMin) {
            throw new CothorityPermissionException("The maximum block size must be bigger or equal to 16000");
        }
        if (newSize > blocksizeMax) {
            throw new CothorityPermissionException("The maximum block size must be smaller than 8*10^6");
        }
        config.setMaxblocksize(newSize);
    }

    /**
     * @return the the protobuf representation of the ReadData
     */
    public ByzCoinProto.ChainConfig toProto() {
        return config.build();
    }
}
