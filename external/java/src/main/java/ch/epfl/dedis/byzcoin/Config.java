package ch.epfl.dedis.byzcoin;


import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.InvalidProtocolBufferException;

import java.time.Duration;

import static java.time.temporal.ChronoUnit.NANOS;

/**
 * Config is the genesis configuration of a ByzCoin ledger. It can be stored only once in ByzCoin
 * and defines the basic running parameters of the ledger and its underlying skipchain.
 */
public class Config {
    private Duration blockInterval;
    private int maxBlockSize;

    /**
     * This instantiates a new configuration to be used in the byzcoin constructor.
     *
     * @param blockInterval - how often the blocks should be created
     */
    public Config(Duration blockInterval){
        this.blockInterval = blockInterval;
    }

    /**
     * Instantiates from an existing protobuf representation.
     */
    public Config(ByzCoinProto.ChainConfig config) {
        this.blockInterval = Duration.of(config.getBlockinterval(), NANOS);
    }

    public Config(byte[] buf) throws CothorityCommunicationException  {
        try {
            ByzCoinProto.ChainConfig config = ByzCoinProto.ChainConfig.parseFrom(buf);
            this.blockInterval = Duration.of(config.getBlockinterval(), NANOS);
            if (! config.hasMaxblocksize()) {
                throw new RuntimeException("no max block size");
            }
            this.maxBlockSize = config.getMaxblocksize();
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

    public int getMaxBlockSize() {
        return maxBlockSize;
    }

    // There is no setter for maxBlockSize right now because we do not expect java clients
    // to need to adjust this. Modifying the block size via a transaction is tested/demoed in Go in
    // TestService_SetConfig.
    //public void setMaxBlockSize(int maxBlockSize) {
    //}

    /**
     * @return the protobuf representation of the configuration.
     */
    public ByzCoinProto.ChainConfig toProto(){
        ByzCoinProto.ChainConfig.Builder b = ByzCoinProto.ChainConfig.newBuilder();
        b.setBlockinterval(blockInterval.get(NANOS));
        b.setMaxblocksize(maxBlockSize);
        return b.build();
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        Config config = (Config) o;
        return blockInterval.equals(config.blockInterval) && maxBlockSize == config.maxBlockSize;
    }
}
