package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;

import java.util.HashSet;
import java.util.Set;

/**
 * Subscription class for ByzCoin. A listener can subscribe to different events and then get notified
 * whenever something happens. This first implementation uses polling to fetch latest blocks and then
 * calls the appropriate receivers. Once we have a streaming service, it will directly connect to the
 * streaming service.
 * <p>
 * - The polling only starts if at least one receiver subscribes.
 * - The polling stops, once the last receiver has been unsubscribed.
 * - Only blocks arriving after the subscription will be passed to the receiver(s).
 * - If more than one block has been received during the subscription period, they will be given in
 * a list to the receiver(s).
 */
public class Subscription {
    /**
     * A SkipBlockReceiver will be informed on any new block arriving.
     */
    public interface SkipBlockReceiver {
        void receive(SkipBlock block);
        void error(String s);
    }

    private ByzCoinRPC bc;
    private AggregateReceiver aggr;
    private ServerIdentity.StreamingConn conn;

    class AggregateReceiver implements SkipBlockReceiver {
        private Set<SkipBlockReceiver> blockReceivers;

        private AggregateReceiver() {
            // TODO concurrent use?
            blockReceivers = new HashSet<>();
        }

        @Override
        public void receive(SkipBlock block) {
            blockReceivers.forEach(br -> br.receive(block));
        }

        @Override
        public void error(String s) {
            blockReceivers.forEach(br -> br.error(s));
        }

        private boolean add(SkipBlockReceiver r) {
            return blockReceivers.add(r);
        }

        private boolean remove(SkipBlockReceiver r) {
            return blockReceivers.remove(r);
        }

        private int size() {
            return blockReceivers.size();
        }
    }

    /**
     * Starts a subscription service, but doesn't call the polling yet.
     *
     * @param bc     a reference to the instantiated ByzCoinRPC
     */
    public Subscription(ByzCoinRPC bc) {
        this.bc = bc;
        this.aggr = new AggregateReceiver();
    }

    /**
     * Subscribes the receiver to the service. After the receiver has been subscribed, every new block will
     * be sent to it. Previously received blocks will not be sent to the receiver.
     *
     * @param br the receiver that wants to be informed of new blocks.
     */
    public void subscribeSkipBlock(SkipBlockReceiver br) throws CothorityCommunicationException  {
        aggr.add(br);
        if (aggr.size() == 1) {
            conn = bc.streamTransactions(aggr);
        }
    }

    /**
     * Unsubscribes an existing receiver from the system. If it was the last receiver, then the polling will
     * stop.
     *
     * @param br the receiver to unsubscribe.
     */
    public void unsubscribeSkipBlock(SkipBlockReceiver br) throws CothorityCommunicationException {
        aggr.remove(br);
        if (aggr.size() == 0) {
            conn.close();
        }
    }

    public boolean isClosed() {
        if (conn == null) {
            return true;
        }
        return conn.isClosed();
    }
}
