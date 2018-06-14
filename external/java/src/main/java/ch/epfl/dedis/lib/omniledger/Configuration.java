package ch.epfl.dedis.lib.omniledger;


import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.proto.OmniLedgerProto;

import java.net.URISyntaxException;
import java.time.Duration;

import static java.time.temporal.ChronoUnit.NANOS;

/**
 * Configuration is the genesis configuration of an omniledger instance. It can be stored only once in omniledger
 * and defines the basic running parameters of omniledger.
 */
public class Configuration {
    private Roster roster;
    private Duration blockInterval;

    /**
     * This instantiates a new configuration to be used in the omniledger constructor.
     *
     * @param r - the roster to be stored in omniledger
     * @param blockInterval - how often the blocks should be created
     */
    public Configuration(Roster r, Duration blockInterval){
        this.roster = r;
        this.blockInterval = blockInterval;
    }

    /**
     * Instantiates from an existing protobuf representation.
     */
    public Configuration(OmniLedgerProto.Configuration proto) throws URISyntaxException{
        this.roster = new Roster(proto.getRoster());
        this.blockInterval = Duration.of(proto.getBlockInterval(), NANOS);
    }

    /**
     * @return the roster stored in that config
     */
    public Roster getRoster(){
        return roster;
    }

    /**
     * @return blockinterval used
     */
    public Duration getBlockInterval(){
        return blockInterval;
    }

    /**
     * @return the protobuf representation of the configuration.
     */
    public OmniLedgerProto.Configuration toProto(){
        OmniLedgerProto.Configuration.Builder config = OmniLedgerProto.Configuration.newBuilder();
        config.setBlockInterval(blockInterval.get(NANOS));
        config.setRoster(roster.toProto());
        return config.build();
    }
}
