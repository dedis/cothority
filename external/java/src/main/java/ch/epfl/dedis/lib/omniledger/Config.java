package ch.epfl.dedis.lib.omniledger;


import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.proto.OmniLedgerProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.time.Duration;

import static java.time.temporal.ChronoUnit.NANOS;

/**
 * Config is the genesis configuration of an omniledger instance. It can be stored only once in omniledger
 * and defines the basic running parameters of omniledger.
 */
public class Config {
    private Duration blockInterval;

    /**
     * This instantiates a new configuration to be used in the omniledger constructor.
     *
     * @param blockInterval - how often the blocks should be created
     */
    public Config(Duration blockInterval){
        this.blockInterval = blockInterval;
    }

    /**
     * Instantiates from an existing protobuf representation.
     */
    public Config(OmniLedgerProto.ChainConfig config) {
        this.blockInterval = Duration.of(config.getBlockinterval(), NANOS);
    }

    public Config(byte[] buf) throws CothorityCommunicationException  {
        try {
            OmniLedgerProto.ChainConfig config = OmniLedgerProto.ChainConfig.parseFrom(buf);
            this.blockInterval = Duration.of(config.getBlockinterval(), NANOS);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
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
    public OmniLedgerProto.ChainConfig toProto(){
        OmniLedgerProto.ChainConfig.Builder b = OmniLedgerProto.ChainConfig.newBuilder();
        b.setBlockinterval(blockInterval.get(NANOS));
        return b.build();
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        Config config = (Config) o;
        return blockInterval.equals(config.blockInterval);
    }
}
