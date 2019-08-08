package ch.epfl.dedis.eventlog;

import ch.epfl.dedis.byzcoin.*;
import ch.epfl.dedis.byzcoin.transaction.*;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.proto.ByzCoinProto;
import ch.epfl.dedis.lib.proto.EventLogProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.*;
import java.util.stream.Collectors;

/**
 * EventLogInstance is for interacting with the eventlog contract on ByzCoin.
 * <p>
 * Contrary to ordinary event logging services, we offer better security and auditability. Below are some of the main
 * features that sets us apart.
 *
 * <ul>
 * <li>
 * Collective witness - a collection of nodes, or conodes, independently observe the logging of an event. The event
 * will only be accepted if a 2/3-majority think it is valid, e.g., the timestamp is reasonable, the client is
 * authorized and so on.
 * </li>
 * <li>
 * Distributed access control - fine-grained client access control with delegation support is configured using
 * DARC.
 * </li>
 * <li>
 * Configurable acceptance criteria - we execute a smart-contract on all nodes, nodes only accept the event if the
 * smart-contract returns a positive result.
 * </li>
 * <li>
 * Existence proof - once an event is logged, an authorized client can request a cryptographic proof (powered by
 * collection) that the event is indeed stored in the blockchain and has not been tampered.
 * </li>
 * </ul>
 */
public class EventLogInstance {
    public static String ContractId = "eventlog";
    public static String LogCmd = "log";
    private Instance instance;
    private ByzCoinRPC bc;
    private HashMap<Integer, Subscription.SkipBlockReceiver> handlers;
    private Random rnd;

    private final static Logger logger = LoggerFactory.getLogger(EventLogInstance.class);

    /**
     * Constructor for when do you not know the eventlog contract, use this constructor when constructing for the first
     * time. This constructor expects the byzcoin RPC to be initialised with a darc that contains "spawn:eventlog".
     *
     * @param bc         the byzcoin RPC
     * @param darcBaseID     the darc ID that has the "spawn:eventlog" permission
     * @param signers    a list of signers that has the "spawn:eventlog" permission
     * @param signerCtrs a list of monotonically increasing counter for every signer
     * @throws CothorityException if something goes wrong
     */
    public EventLogInstance(ByzCoinRPC bc, DarcId darcBaseID, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        this.bc = bc;
        InstanceId id = this.initEventlogInstance(darcBaseID, signers, signerCtrs);

        // wait for byzcoin to commit the transaction in block
        try {
            Thread.sleep(5 * bc.getConfig().getBlockInterval().toMillis());
        } catch (InterruptedException e) {
            throw new CothorityException(e);
        }
        this.setInstance(id);
        this.handlers = new HashMap<>();
        this.rnd = new Random();
    }

    /**
     * Constructor for when the caller already knows the eventlog contract.
     *
     * @param bc the byzcoin RPC
     * @param id the contract ID, it must be already initialised and stored on byzcoin
     * @throws CothorityException if something goes wrong
     */
    public EventLogInstance(ByzCoinRPC bc, InstanceId id) throws CothorityException {
        this.bc = bc;
        this.setInstance(id);
    }

    /**
     * Logs a list of events, the returned value is a list of ID for every event which can be used to retrieve events
     * later. Note that when the function returns, it does not mean the event is stored successfully in a block, use the
     * get function to verify that the event is actually stored.
     *
     * @param events     a list of events to log
     * @param signers    a list of signers with the permission "invoke:eventlog.log"
     * @param signerCtrs a list of monotonically increasing counter for every signer
     * @return a list of keys which can be used to retrieve the logged events
     * @throws CothorityException if something goes wrong
     */
    public List<InstanceId> log(List<Event> events, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        Pair<ClientTransaction, List<InstanceId>> txAndKeys = makeTx(events, signers, signerCtrs);
        bc.sendTransaction(txAndKeys._1);
        return txAndKeys._2;
    }

    /**
     * Logs an event, the returned value is the ID of the event which can be retrieved later. Note that when this
     * function returns, it does not mean the event is stored successfully in a block, use the get function to verify
     * that the event is actually stored.
     *
     * @param event      the event to log
     * @param signers    a list of signers that has the "invoke:eventlog.log" permission
     * @param signerCtrs a list of monotonically increasing counter for every signer
     * @return the key which can be used to retrieve the event later
     * @throws CothorityException if something goes wrong
     */
    public InstanceId log(Event event, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        return this.log(Collections.singletonList(event), signers, signerCtrs).get(0);
    }

    /**
     * Retrieves the stored event by key. An exception is thrown when if the event does not exist.
     *
     * @param key the key for which the event is stored
     * @return The event if it is found.
     * @throws CothorityException if something goes wrong
     */
    public Event get(InstanceId key) throws CothorityException {
        Proof p = bc.getProofFromLatest(key);
        if (!p.exists(key.getId())) {
            throw new CothorityCryptoException("event does not exist");
        }
        StateChangeBody body = p.getValues();
        try {
            EventLogProto.Event event = EventLogProto.Event.parseFrom(body.getValue());
            return new Event(event);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Searches for events based on topic and a time range. If the topic is an empty string, all topics within that
     * range are returned (from &lt; when &lt;= to). The query may not return all events, this is indicated by the truncated
     * flag in the return value.
     *
     * @param topic the topic to search, if it is an empty string, all topics are included, we do not support regex
     * @param from  the start of the search range (exclusive).
     * @param to    the end of the search range (inclusive).
     * @return a list of events and a flag indicating whether the result is truncated
     * @throws CothorityException if something goes wrong
     */
    public SearchResponse search(String topic, long from, long to) throws CothorityException {
        // Note: this method is a bit different from the others, we directly use the raw sendMessage instead of via
        // ByzCoinRPC.
        EventLogProto.SearchRequest.Builder b = EventLogProto.SearchRequest.newBuilder();
        b.setInstance(ByteString.copyFrom(this.instance.getId().getId()));
        b.setId(this.bc.getGenesisBlock().getId().toProto());
        b.setTopic(topic);
        b.setFrom(from);
        b.setTo(to);

        ByteString msg = this.bc.getRoster().sendMessage("EventLog/SearchRequest", b.build());

        try {
            EventLogProto.SearchResponse resp = EventLogProto.SearchResponse.parseFrom(msg);
            return new SearchResponse(resp);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * This is the interface that must be used for handling new events.
     */
    public interface EventHandler {
        void process(Event e, byte[] id);
        void error(String s);
    }

    class EventLogReceiver implements Subscription.SkipBlockReceiver {
        private EventHandler handler;
        EventLogReceiver(EventHandler h) {
            handler = h;
        }

        @Override
        public void receive(SkipBlock block) {
            // check the header is correct
            try {
                ByzCoinProto.DataHeader.parseFrom(block.getData());
            } catch (InvalidProtocolBufferException e) {
                handler.error(e.getMessage());
                return;
            }
            // parse the payload
            DataBody body;
            try {
                body = new DataBody(ByzCoinProto.DataBody.parseFrom(block.getPayload()));
            } catch (InvalidProtocolBufferException e) {
                handler.error(e.getMessage());
                return;
            }

            // parse the transactions
            for (TxResult tx : body.getTxResults()) {
                if (!tx.isAccepted()){
                    continue;
                }
                for (Instruction instr : tx.getClientTransaction().getInstructions()) {
                    if (instr.getInvoke() == null) {
                        continue;
                    }
                    if (!instr.getInvoke().getContractID().equals(ContractId) || !instr.getInvoke().getCommand().equals(LogCmd)) {
                        continue;
                    }
                    // try to find the event argument
                    Optional<Argument> opArg = instr.getInvoke().getArguments().stream()
                            .filter(x -> x.getName().equals("event"))
                            .findFirst();
                    if (!opArg.isPresent()) {
                        continue;
                    }
                    // parse the event argument
                    Event event;
                    try {
                        event = new Event(EventLogProto.Event.parseFrom(opArg.get().getValue()));
                    } catch (InvalidProtocolBufferException e) {
                        handler.error(e.getMessage());
                        continue;
                    }
                    handler.process(event, block.getId().getId());
                }
            }
        }

        @Override
        public void error(String s) {
            handler.error(s);
        }
    }

    /**
     * Concurrent class.
     */
    class BufferedEventLogReceiver extends EventLogReceiver {
        class Pair {
            final SkipBlock block;
            final String error;
            Pair(SkipBlock b, String s) {
                block = b;
                error = s;
            }
        }

        List<Pair> buffer;
        Boolean flushed = false;

        BufferedEventLogReceiver(EventHandler h) {
            super(h);
            buffer = new ArrayList<>();
        }

        synchronized void flush(List<SkipBlock> prepend) {
            SkipBlock current = null;
            for (SkipBlock b : prepend) {
                current = b;
                super.receive(b);
            }

            for (Pair p : buffer) {
                if (p.block == null) {
                    super.error(p.error);
                }
                else if (p.block.getBackLinks().size() == 0) {
                    super.error("no back links");
                }
                else if (current != null && Arrays.equals(p.block.getBackLinks().get(0).getId(), current.getHash())) {
                    super.receive(p.block);
                } else {
                    // do nothing because this is a duplicate block
                }
            }
            flushed = true;
        }

        @Override
        public synchronized void receive(SkipBlock block) {
            if (flushed) {
                super.receive(block);
            } else {
                buffer.add(new Pair(block, null));
            }
        }

        @Override
        public synchronized void error(String s) {
            if (flushed) {
                super.error(s);
            } else {
                buffer.add(new Pair(null, s));
            }
        }
    }

    /**
     * Register the handler to listen to new events.
     * If an error is thrown, the caller should unsubscribe the tag.
     *
     * @param handler is the event handler, it will be called in a different thread.
     * @return an integer tag that can be used to unregister the handler.
     * @throws CothorityCommunicationException when the server fails or refuses to accept the subscription.
     */
    public int subscribeEvents(EventHandler handler) throws CothorityCommunicationException {
        EventLogReceiver r = new EventLogReceiver(handler);
        int tag = rnd.nextInt();
        this.handlers.put(tag, r);
        this.bc.subscribeSkipBlock(r);
        return tag;
    }

    /**
     * Register the handler to listen to new events from the given block.
     * The function blocks while it reads old events from the given block.
     * If an error is thrown, the caller should unsubscribe the tag.
     *
     * @param handler is the event handler, it will be called in a different thread.
     * @param from is the block which marks the beginning of the subscription.
     * @return an integer tag that can be used to unregister the handler.
     * @throws CothorityCommunicationException
     * @throws CothorityCryptoException
     */
    public int subscribeEvents(EventHandler handler, byte[] from) throws CothorityCommunicationException, CothorityCryptoException {
        // 1. stream to a buffer (because we don't know which ones will be duplicates yet)
        BufferedEventLogReceiver r = new BufferedEventLogReceiver(handler);
        int tag = rnd.nextInt();
        this.handlers.put(tag, r);
        this.bc.subscribeSkipBlock(r);

        // 2. use GetUpdateChain to find the missing events and call handler
        // if the skipblock ID is wrong or does not exist the function will throw an exception
        List<SkipBlock> blocks = this.bc.getSkipchain().getUpdateChain(new SkipblockId(from));

        // 3. read from the buffer, remove duplicates and call the handler
        r.flush(blocks);

        return tag;
    }

    /**
     * Unsubscribe a handler.
     *
     * @param tag is the ID for the handler, it is created in subscribeEvents.
     */
    public void unsubscribeEvents(int tag) {
        Subscription.SkipBlockReceiver sbr = this.handlers.get(tag);
        this.handlers.remove(tag);
        this.bc.unsubscribeBlock(sbr);
    }

    /**
     * Gets the contract ID which can be stored to re-connect to the same eventlog instance in the future.
     *
     * @return the contract ID
     */
    public InstanceId getInstanceId() {
        return instance.getId();
    }

    /**
     * Constructor for when the caller already knows the eventlog contract.
     *
     * @param bc the byzcoin RPC
     * @param id the contract ID, it must be already initialised and stored on byzcoin
     * @return a new EventLogInstance
     * @throws CothorityException if something goes wrong
     */
    public static EventLogInstance fromByzcoin(ByzCoinRPC bc, InstanceId id) throws CothorityException {
        return new EventLogInstance(bc, id);
    }

    private InstanceId initEventlogInstance(DarcId darcBaseID, List<Signer> signers, List<Long> signerCtrs) throws CothorityException {
        if (this.instance != null) {
            throw new CothorityException("already have a contract");
        }
        Spawn spawn = new Spawn(ContractId, new ArrayList<>());
        Instruction instr = new Instruction(new InstanceId(darcBaseID.getId()),
                signers.stream().map(Signer::getIdentity).collect(Collectors.toList()),
                signerCtrs,
                spawn);

        ClientTransaction tx = new ClientTransaction(Collections.singletonList(instr), bc.getProtocolVersion());
        tx.signWith(signers);
        bc.sendTransaction(tx);

        return tx.getInstructions().get(0).deriveId("");
    }

    private void setInstance(InstanceId id) throws CothorityException {
        Instance inst = Instance.fromByzcoin(bc, id);
        if (!inst.getContractId().equals(ContractId)) {
            logger.error("wrong contract: {}", inst.getContractId());
            throw new CothorityNotFoundException("this is not an eventlog contract");
        }
        this.instance = inst;
        logger.info("new eventlog contract: " + inst.getId().toString());
    }

    private static final class Pair<A, B> {
        A _1;
        B _2;

        private Pair(A a, B b) {
            this._1 = a;
            this._2 = b;
        }
    }

    private Pair<ClientTransaction, List<InstanceId>> makeTx(List<Event> events, List<Signer> signers, List<Long> signerCtrs)
            throws CothorityException {
        List<Instruction> instrs = new ArrayList<>();
        List<InstanceId> keys = new ArrayList<>();
        for (Event e : events) {
            List<Argument> args = new ArrayList<>();
            args.add(new Argument("event", e.toProto().toByteArray()));
            Invoke invoke = new Invoke(ContractId, LogCmd, args);
            Instruction instr = new Instruction(instance.getId(),
                    signers.stream().map(Signer::getIdentity).collect(Collectors.toList()),
                    signerCtrs,
                    invoke);
            instrs.add(instr);
            signerCtrs = incrementCtrs(signerCtrs);
        }
        ClientTransaction tx = new ClientTransaction(instrs, bc.getProtocolVersion());
        tx.signWith(signers);
        tx.getInstructions().forEach(instr -> keys.add(instr.deriveId("")));
        return new Pair<>(tx, keys);
    }

    private static List<Long> incrementCtrs(List<Long> xs) {
        List<Long> out = new ArrayList<>(xs);
        for (int i = 0; i < out.size(); i++) {
            out.set(i, out.get(i) + 1);
        }
        return out;
    }
}
