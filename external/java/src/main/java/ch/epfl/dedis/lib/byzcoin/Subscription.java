package ch.epfl.dedis.lib.byzcoin;

import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.skipchain.SkipchainRPC;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ScheduledFuture;

import static java.util.concurrent.TimeUnit.MILLISECONDS;

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
    @FunctionalInterface
    public interface SkipBlockReceiver {
        void receive(List<SkipBlock> blocks);
    }

    private SkipchainRPC sc;
    private Set<SkipBlockReceiver> blockReceivers = new HashSet<>();
    private final ScheduledExecutorService scheduler =
            Executors.newScheduledThreadPool(1);
    private Runnable pollRunnable;
    private ScheduledFuture<?> pollHandle;
    private SkipBlock latestBlock;
    private long millis;

    /**
     * Starts a subscription service, but doesn't call the polling yet.
     *
     * @param sc     a reference to the instantiated skipchainRPC
     * @param millis how many millisecond to wait between two polling events.
     * @throws CothorityException
     */
    public Subscription(SkipchainRPC sc, long millis) throws CothorityException {
        this.sc = sc;
        this.millis = millis;
        pollRunnable = () -> poll();
    }

    /**
     * Subscribes the receiver to the service. After the receiver has been subscribed, every new block will
     * be sent to it. Previously received blocks will not be sent to the receiver.
     *
     * @param br the receiver that wants to be informed of new blocks.
     */
    public void subscribeSkipBlock(SkipBlockReceiver br) {
        blockReceivers.add(br);
        if (blockReceivers.size() == 1) {
            startPolling();
        }
    }

    /**
     * Unsubscribes an existing receiver from the system. If it was the last receiver, then the polling will
     * stop.
     *
     * @param br the receiver to unsubscribe.
     */
    public void unsubscribeSkipBlock(SkipBlockReceiver br) {
        blockReceivers.remove(br);
        if (blockReceivers.size() == 0) {
            stopPolling();
        }
    }

    private void poll() {
        List<SkipBlock> newBlocks = new ArrayList<>();
        try {
            // Update the latest block
            latestBlock = sc.getSkipblock(latestBlock.getId());
            while (latestBlock.getForwardLinks().size() > 0) {
                // Get next block with link-height 0
                latestBlock = sc.getSkipblock(latestBlock.getForwardLinks().get(0).getTo());
                newBlocks.add(latestBlock);
            }
        } catch (CothorityException e) {
            return;
        }
        if (newBlocks.size() > 0) {
            blockReceivers.forEach(br -> br.receive(newBlocks));
        }
    }

    private void startPolling() {
        if (pollHandle == null) {
            try {
                latestBlock = sc.getLatestSkipblock();
            } catch (CothorityException e) {
            }
            pollHandle = scheduler.scheduleWithFixedDelay(pollRunnable, 0,
                    this.millis, MILLISECONDS);
        }
    }

    private void stopPolling() {
        if (pollHandle == null) {
            return;
        }
        pollHandle.cancel(false);
        try {
            pollHandle.wait();
            pollHandle = null;
        } catch (InterruptedException e) {
        }
    }
}
