package ch.epfl.dedis.lib.omniledger.contracts;

import ch.epfl.dedis.lib.eventlog.SearchResponse;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import ch.epfl.dedis.lib.omniledger.darc.Request;
import ch.epfl.dedis.lib.omniledger.darc.Signer;
import ch.epfl.dedis.lib.eventlog.Event;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.omniledger.*;
import ch.epfl.dedis.lib.omniledger.darc.Signature;
import ch.epfl.dedis.proto.EventLogProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

/**
 * Workflow:
 * In order to instantiate an EventLogInstance, one needs to hold an OmniledgerRPC and an instance which contains
 */
public class EventLogInstance {
    private Instance instance;
    private OmniledgerRPC ol;

    private final static Logger logger = LoggerFactory.getLogger(EventLogInstance.class);

    public EventLogInstance(OmniledgerRPC ol, InstanceId id) throws CothorityException {
        this.ol = ol;
        Proof p = ol.getProof(id);
        instance = new Instance(p);
        if (!instance.getContractId().equals("eventlog")) {
            logger.error("wrong instance: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not an eventlog instance");
        }
    }

    public List<byte[]> log(List<Event> events, List<Signer> signers) throws CothorityException {
        ClientTransaction tx = makeTx(events, this.instance.getId().getDarcId(), signers);
        ol.sendTransaction(tx);
        return tx.getInstructions()
                .stream()
                .map(instr -> instr.getInstId().getId())
                .collect(Collectors.toList());
    }

    public byte[] log(Event event, List<Signer> signers) throws CothorityException {
        List<Event> events = new ArrayList<>();
        events.add(event);
        return this.log(events, signers).get(0);
    }

    public Event get(byte[] key) throws CothorityException {
        Proof p = ol.getProof(this.instance.getId());
        if (!p.matches()) {
            throw new CothorityCryptoException("key does not exist");
        }
        if (!Arrays.equals(p.getKey(), key)) {
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

    public SearchResponse search(String topic, long from, long to) throws CothorityException {
        // TODO this is a bit different from the others, we directly use the raw sendMessage
        EventLogProto.SearchRequest.Builder b = EventLogProto.SearchRequest.newBuilder();
        b.setId(this.instance.getId().toByteString());
        b.setTopic(topic);
        b.setFrom(from);
        b.setTo(to);

        ByteString msg =  this.ol.getConfig().getRoster().sendMessage("EventLog/SearchRequest", b.build());

        try {
            EventLogProto.SearchResponse resp = EventLogProto.SearchResponse.parseFrom(msg);
            return new SearchResponse(resp);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    private static ClientTransaction makeTx(List<Event> events, DarcId darcID, List<Signer> signers) throws CothorityCryptoException {
        List<Signature> sigs = new ArrayList<>();
        for (Signer signer : signers) {
            sigs.add(new Signature(new byte[0], signer.getIdentity()));
        }

        byte[] instrNonce = genNonce();
        List<Instruction> instrs = new ArrayList<>();
        int idx = 0;
        for (Event e : events) {
            InstanceId objId = new InstanceId(darcID, SubId.random());
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
