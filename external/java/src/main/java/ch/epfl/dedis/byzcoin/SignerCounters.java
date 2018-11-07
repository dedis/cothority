package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

import java.util.ArrayList;
import java.util.List;

/**
 * This class represents the result from the getSignerCounters RPC call.
 */
public class SignerCounters {
    private List<Long> counters;

    /**
     * Constructor for a list of counters
     */
    public SignerCounters(List<Long> counters) {
        this.counters = counters;
    }

    /**
     * Constructor for the protobuf representation
     */
    public SignerCounters(ByzCoinProto.GetSignerCountersResponse proto) {
        this.counters = new ArrayList<>();
        this.counters.addAll(proto.getCountersList());
    }

    /**
     * Getter for the counters
     */
    public List<Long> getCounters() {
        return counters;
    }

    /**
     * Increments every counter
     */
    public void increment() {
        for (int i = 0; i < counters.size(); i++) {
            counters.set(i, counters.get(i) + 1);
        }
    }

    /**
     * Get the first counter
     */
    public Long head() {
        return this.counters.get(0);
    }
}
