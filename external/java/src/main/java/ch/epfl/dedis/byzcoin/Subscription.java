package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.network.ServerIdentity;

import java.util.HashSet;
import java.util.Set;

/**
 * Subscription class for ByzCoin. A listener can subscribe to different events and then get notified
 * whenever something happens via the streaming service. We only maintain one connection because we can
 * replicate the events to all the subscribers.
 * <p>
 * - The connection is made if at least one receiver subscribes.
 * - The connection stops once the last receiver has been unsubscribed.
 * - The subscribers will only see events (blocks) that arrive after the subscription is made.
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

    /**
     * To reduce the number of connections, we create this aggregate receiver that replicates events to all the
     * subscribers.
     */
    class AggregateReceiver implements SkipBlockReceiver {
        private Set<SkipBlockReceiver> blockReceivers;

        private AggregateReceiver() {
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
     * @param bc a reference to the instantiated ByzCoinRPC
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
     * @throws CothorityCommunicationException is something went wrong
     */
    public void subscribeSkipBlock(SkipBlockReceiver br) throws CothorityCommunicationException {
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
    public void unsubscribeSkipBlock(SkipBlockReceiver br) {
        aggr.remove(br);
        if (aggr.size() == 0) {
            conn.close();
        }
    }

    /**
     * Checks whether the connection is closed.
     * @return true if closed
     */
    public boolean isClosed() {
        if (conn == null) {
            return true;
        }
        return conn.isClosed();
    }
}
