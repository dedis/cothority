package ch.epfl.dedis.lib.omniledger.contracts;

import ch.epfl.dedis.lib.eventlog.SearchResponse;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import ch.epfl.dedis.lib.omniledger.darc.Signer;
import ch.epfl.dedis.lib.eventlog.Event;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.omniledger.*;
import ch.epfl.dedis.proto.EventLogProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * EventLogInstance is for interacting with the eventlog contract on OmniLedger.
 * <p>
 * Contrary to ordinary event logging services, we offer better security and auditability. Below are some of the main
 * features that sets us apart.
 * <p>
 * <ul>
 *  <li>
 *      Collective witness - a collection of nodes, or conodes, indepdently observe the logging of an event. The event
 *      will only be accepted if a 2/3-majority think it is valid, e.g., the timestamp is reasonable, the client is
 *      authorised and so on.
 *  </li>
 *  <li>
 *      Distributed access control - fine-grained client access control with delegation support is configured using
 *      DARC.
 *  </li>
 *  <li>
 *     Configurable acceptance criteria - we execute a smart-contract on all nodes, nodes only accept the event if the
 *     smart-contract returns a positive result.
 *  </li>
 *
 *  <li>
 *     Existance proof - once an event is logged, an authorised client can request a cryptographic proof (powered by
 *     collection) that the event is indeed stored in the blockchain and has not been tampered.
 *  </li>
 * </ul>
 */
public class EventLogInstance {
    private Instance instance;
    private OmniledgerRPC ol;

    private final static Logger logger = LoggerFactory.getLogger(EventLogInstance.class);

    /**
     * Constructor for when do you not know the eventlog instance, use this constructor when constructing for the first
     * time. This constructor expects the omniledger RPC to be initialised with a darc that contains "spawn:eventlog".
     * @param ol the omniledger RPC
     * @param signers a list of signers that has the "spawn:eventlog" permission
     * @param darcId the darc ID that has the "spawn:eventlog" permission
     * @throws CothorityException
     */
    public EventLogInstance(OmniledgerRPC ol, List<Signer> signers, DarcId darcId) throws CothorityException {
        this.ol = ol;
        InstanceId id = this.initEventlogInstance(signers, darcId);

        // wait for omniledger to commit the transaction in block
        try {
            Thread.sleep(5 * ol.getConfig().getBlockInterval().toMillis());
        } catch (InterruptedException e) {
            throw new CothorityException(e);
        }

        this.instance = getInstaceProof(id);
    }

    /**
     * Constructor for when the caller already knows the eventlog instance.
     * @param ol the omniledger RPC
     * @param id the instance ID, it must be already initialised and stored on omniledger
     * @throws CothorityException
     */
    public EventLogInstance(OmniledgerRPC ol, InstanceId id) throws CothorityException {
        this.ol = ol;
        this.instance = getInstaceProof(id);
    }

    /**
     * Logs a list of events, the returned value is a list of ID for every event which can be used to retrieve events
     * later. Note that when the function returns, it does not mean the event is stored successfully in a block, use the
     * get function to verify that the event is actually stored.
     * @param events a list of events to log
     * @param signers a list of signers with the permission "invoke:eventlog"
     * @return a list of keys which can be used to retrieve the logged events
     * @throws CothorityException
     */
    public List<InstanceId> log(List<Event> events, List<Signer> signers) throws CothorityException {
        Pair<ClientTransaction, List<InstanceId>> txAndKeys = makeTx(events, this.instance.getId(), signers);
        ol.sendTransaction(txAndKeys._1);
        return txAndKeys._2;
    }

    /**
     * Logs an event, the returned value is the ID of the event which can be retrieved later. Note that when this
     * function returns, it does not mean the event is stored successfully in a block, use the get function to verify
     * that the event is actually stored.
     * @param event the event to log
     * @param signers a list of signers that has the "invoke:eventlog" permission
     * @return the key which can be used to retrieve the event later
     * @throws CothorityException
     */
    public InstanceId log(Event event, List<Signer> signers) throws CothorityException {
        return this.log(Arrays.asList(event), signers).get(0);
    }

    /**
     * Retrieves the stored event by key. An exception is thrown when if the event does not exist.
     * @param key the key for which the event is stored
     * @return The event if it is found.
     * @throws CothorityException
     */
    public Event get(InstanceId key) throws CothorityException {
        Proof p = ol.getProof(key);
        if (!p.matches()) {
            throw new CothorityCryptoException("key does not exist");
        }
        if (!Arrays.equals(p.getKey(), key.getId())) {
            throw new CothorityCryptoException("wrong key");
        }
        if (p.getValues().size() < 2) {
            throw new CothorityCryptoException("not enough values");
        }
        try {
            EventLogProto.Event event = EventLogProto.Event.parseFrom(p.getValues().get(0));
            return new Event(event);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Searches for events based on topic and a time range. If the topic is an empty string, all topics within that
     * range are returned (from < when <= to). The query may not return all events, this is indicated by the truncated
     * flag in the return value.
     * @param topic the topic to search, if it is an empty string, all topics are included, we do not support regex
     * @param from the start of the search range (exclusive).
     * @param to the end of the search range (inclusive).
     * @return a list of events and a flag indicating whether the result is truncated
     * @throws CothorityException
     */
    public SearchResponse search(String topic, long from, long to) throws CothorityException {
        // Note: this method is a bit different from the others, we directly use the raw sendMessage instead of via
        // OmniLedgerRPC.
        EventLogProto.SearchRequest.Builder b = EventLogProto.SearchRequest.newBuilder();
        b.setEventlogid(this.instance.getId().toProto());
        b.setId(this.ol.getGenesis().getId().toProto());
        b.setTopic(topic);
        b.setFrom(from);
        b.setTo(to);

        ByteString msg =  this.ol.getRoster().sendMessage("EventLog/SearchRequest", b.build());

        try {
            EventLogProto.SearchResponse resp = EventLogProto.SearchResponse.parseFrom(msg);
            return new SearchResponse(resp);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Gets the instance ID which can be stored to re-connect to the same eventlog instance in the future.
     * @return the instance ID
     */
    public InstanceId getInstanceId() {
        return instance.getId();
    }

    private InstanceId initEventlogInstance(List<Signer> signers, DarcId darcId) throws CothorityException {
        if (this.instance != null) {
            throw new CothorityException("already have an instance");
        }
        Spawn spawn = new Spawn("eventlog", new ArrayList<>());
        InstanceId darcInstId = new InstanceId(darcId, new SubId(new byte[32]));
        Instruction instr = new Instruction(darcInstId, genNonce(), 0, 1, spawn);
        instr.signBy(signers);

        ClientTransaction tx = new ClientTransaction(Arrays.asList(instr));
        ol.sendTransaction(tx);

        SubId subId = new SubId(instr.hash());
        return new InstanceId(darcId, subId);
    }


    private Instance getInstaceProof(InstanceId id) throws CothorityException {
        Proof p = ol.getProof(id);
        Instance inst = new Instance(p);
        if (!inst.getContractId().equals("eventlog")) {
            logger.error("wrong instance: {}", inst.getContractId());
            throw new CothorityNotFoundException("this is not an eventlog instance");
        }
        logger.info("new eventlog instance: " + inst.getId().toString());
        return inst;
    }

    private static final class Pair<A, B> {
        A _1;
        B _2;

        private Pair(A a, B b) {
            this._1 = a;
            this._2 = b;
        }
    }

    private static Pair<ClientTransaction, List<InstanceId>> makeTx(List<Event> events, InstanceId instId, List<Signer> signers) throws CothorityCryptoException {
        byte[] instrNonce = genNonce();
        List<Instruction> instrs = new ArrayList<>();
        List<InstanceId> keys = new ArrayList<>();
        int idx = 0;
        for (Event e : events) {
            List<Argument> args = new ArrayList<>();
            args.add(new Argument("event", e.toProto().toByteArray()));
            Invoke invoke = new Invoke("eventlog", args);
            Instruction instr = new Instruction(instId, instrNonce, idx, events.size(), invoke);
            instr.signBy(signers);
            instrs.add(instr);
            keys.add(instr.deriveId("event"));
            idx++;
        }
        ClientTransaction tx = new ClientTransaction(instrs);
        return new Pair(tx, keys);
    }

    private static byte[] genNonce()  {
        SecureRandom sr = new SecureRandom();
        byte[] nonce  = new byte[32];
        sr.nextBytes(nonce);
        return nonce;
    }
}
