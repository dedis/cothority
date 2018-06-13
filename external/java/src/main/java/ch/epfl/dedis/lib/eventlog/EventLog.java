package ch.epfl.dedis.lib.eventlog;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.omniledger.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.omniledger.*;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import ch.epfl.dedis.proto.EventLogProto;

import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * EventLog is the client class for interacting with the EventLog service.
 */
public class EventLog {
    private SkipblockId genesis;
    private Roster roster;
    private Darc darc;
    private List<Signer> signers;

    /***
     * Construct an EventLog client.
     * @param roster The roster for the servers, check the Roster class documentation for the different ways of creating it.
     * @param signers The list of signers that are authorised by the service to log events.
     * @param blockInterval The blockInterval in nanoseconds.
     * @throws CothorityCommunicationException
     * @throws CothorityCryptoException
     */
    public EventLog(Roster roster, List<Signer> signers, long blockInterval) throws CothorityCommunicationException, CothorityCryptoException {
        List<Identity> identities = new ArrayList<>();
        for (Signer signer : signers) {
            identities.add(signer.getIdentity());
        }
        Map<String, byte[]> rules = Darc.initRules(identities, new ArrayList<>());
        rules.put("spawn:eventlog", rules.get("invoke:evolve"));

        Darc darc = new Darc(rules, "eventlog owner".getBytes());
        byte[] genesisBuf  = this.init(roster, darc, blockInterval);

        this.genesis = new SkipblockId(genesisBuf);
        this.roster = roster;
        this.darc = darc;
        this.signers = signers;
    }

    /***
     * Logs an event, the returned value is the ID of the event which can be retrieved later. Note that when this
     * function returns, it does not mean the event is stored successfully in a block, use the get function to verify
     * that the event is actually stored.
     * @param event An event to log.
     * @return An ID for the event.
     * @throws CothorityCommunicationException
     * @throws CothorityCryptoException
     */
    public byte[] log(Event event) throws CothorityCommunicationException, CothorityCryptoException {
        List<Event> events = new ArrayList<>();
        events.add(event);
        return this.log(events).get(0);
    }

    /***
     * Logs a set of events, it returns an ID for every event. Note that when this function returns, it does not mean
     * the event is stored successfully in a block, use the get function to verify that the event is actually stored.
     * @param events A list of events for logging.
     * @return A list of IDs for every event, in the same order.
     * @throws CothorityCommunicationException
     * @throws CothorityCryptoException
     */
    public List<byte[]> log(List<Event> events) throws CothorityCommunicationException, CothorityCryptoException {
        ClientTransaction tx = makeTx(events, this.darc.getBaseId(), this.signers);
        EventLogProto.LogRequest.Builder b = EventLogProto.LogRequest.newBuilder();
        b.setSkipchainid(ByteString.copyFrom(this.genesis.getId()));
        b.setTransaction(tx.toProto());

        ByteString msg = this.roster.sendMessage("EventLog/LogRequest", b.build());

        try {
            // try to parse the empty response to see if it's valid
            EventLogProto.LogResponse.parseFrom(msg);
            List<byte[]> out = new ArrayList<>();
            for (Instruction instr : tx.getInstructions()) {
                out.add(instr.getObjId().toByteArray());
            }
            return out;
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /***
     * Retrieves the stored event by key.
     * @param key The key (ID) of the event, usually returned by log.
     * @return The value of the event.
     * @throws CothorityCommunicationException
     */
    public Event get(byte[] key) throws CothorityCommunicationException {
        EventLogProto.GetEventRequest.Builder b = EventLogProto.GetEventRequest.newBuilder();
        b.setSkipchainid(ByteString.copyFrom(this.genesis.getId())); b.setKey(ByteString.copyFrom(key));

        ByteString msg = this.roster.sendMessage("EventLog/GetEventRequest", b.build());

        try {
            EventLogProto.Event event = EventLogProto.GetEventResponse.parseFrom(msg).getEvent();
            return new Event(event.getWhen(), event.getTopic(), event.getContent());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    private byte[] init(Roster roster, Darc ownerDarc, long blockInterval) throws CothorityCommunicationException  {
        EventLogProto.InitRequest.Builder b = EventLogProto.InitRequest.newBuilder();
        b.setRoster(roster.toProto());
        b.setOwner(ownerDarc.toProto());
        b.setBlockinterval(blockInterval);

        ByteString msg = roster.sendMessage("EventLog/InitRequest", b.build());

        try {
            EventLogProto.InitResponse reply = EventLogProto.InitResponse.parseFrom(msg);
            return reply.getId().toByteArray();
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    private static ClientTransaction makeTx(List<Event> events, DarcId darcID, List<Signer> signers) throws CothorityCryptoException {
        List<Signature> sigs = new ArrayList<>();
        for (Signer signer : signers) {
            sigs.add(new Signature(new byte[0], signer.getIdentity()));
        }

        byte[] instrNonce = EventLog.genNonce();
        List<Instruction> instrs = new ArrayList<>();
        int idx = 0;
        for (Event e : events) {
            ObjectID objId = new ObjectID(darcID, EventLog.genNonce());
            List<Argument> args = new ArrayList<>();
            args.add(new Argument("event", e.toProto().toByteArray()));
            Spawn spawn = new Spawn("eventlog", args);
            Instruction instr = new Instruction(objId, instrNonce, idx, events.size(), spawn);
            instr.setSignatures(sigs);
            instrs.add(instr);
            idx++;
        }

        for (Instruction instr : instrs) {
            List<Signature> darcSigs = new ArrayList<>();
            for (Signer signer : signers) {
                Request dr = instr.toDarcRequest();
                byte[] sig;
                try {
                    sig = signer.sign(dr.hash());
                } catch (Signer.SignRequestRejectedException e) {
                    throw new CothorityCryptoException(e.getMessage());
                }
                darcSigs.add(new Signature(sig, signer.getIdentity()));
            }
            instr.setSignatures(darcSigs);
        }

        return new ClientTransaction(instrs);
    }

    private static byte[] genNonce()  {
        SecureRandom sr = new SecureRandom();
        byte[] nonce  = new byte[32];
        sr.nextBytes(nonce);
        return nonce;
    }
}
