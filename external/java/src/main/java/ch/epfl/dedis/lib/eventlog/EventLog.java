package ch.epfl.dedis.lib.eventlog;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.omniledger.*;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import ch.epfl.dedis.proto.EventLogProto;

import javax.xml.bind.DatatypeConverter;
import javax.xml.crypto.Data;
import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public class EventLog {
    private SkipblockId genesis;
    private Roster roster;
    private Darc darc;
    private List<Signer> signers;
    // private static int version = 1;
    // private final Logger logger = LoggerFactory.getlogger(EventLog.class);

    public EventLog(Roster roster, Signer signer) throws CothorityCommunicationException, CothorityCryptoException {
        List<Identity> identities = new ArrayList<>();
        identities.add(signer.getIdentity());
        Map<String, byte[]> rules = Darc.initRules(identities, new ArrayList<>());
        rules.put("Spawn_eventlog", rules.get("_evolve"));

        Darc darc = new Darc(rules, "eventlog owner".getBytes());
        byte[] genesisBuf  = this.init(roster, darc);

        this.genesis = new SkipblockId(genesisBuf);
        this.roster = roster;
        this.darc = darc;

        // TODO constructor should be able to support multiple signers
        this.signers = new ArrayList<>();
        this.signers.add(signer);
    }

    private byte[] init(Roster roster, Darc ownerDarc) throws CothorityCommunicationException  {
        EventLogProto.InitRequest.Builder b = EventLogProto.InitRequest.newBuilder();
        b.setRoster(roster.toProto());
        b.setOwner(ownerDarc.toProto());

        ByteString msg = roster.sendMessage("EventLog/InitRequest", b.build());

        try {
            EventLogProto.InitResponse reply = EventLogProto.InitResponse.parseFrom(msg);
            return reply.getId().toByteArray();
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    public byte[] log(Event event) throws CothorityCommunicationException, CothorityCryptoException {
        List<Event> events = new ArrayList<>();
        events.add(event);
        return this.log(events).get(0);
    }

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
