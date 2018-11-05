package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.proto.ByzCoinProto;

import java.util.ArrayList;
import java.util.List;

public class SignerCounters {
    private List<Long> counters;

    public SignerCounters(List<Long> counters) {
        this.counters = counters;
    }

    public SignerCounters(ByzCoinProto.GetSignerCountersResponse proto) {
        this.counters = new ArrayList<>();
        this.counters.addAll(proto.getCountersList());
    }

    public List<Long> getCounters() {
        return counters;
    }

    public void increment() {
        for (int i = 0; i < counters.size(); i++) {
            counters.set(i, counters.get(i) + 1);
        }
    }

    public Long head() {
        return this.counters.get(0);
    }
}
