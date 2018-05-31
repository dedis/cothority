package ch.epfl.dedis.lib.eventlog;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipBlock;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Identity;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.TransactionProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import ch.epfl.dedis.proto.EventLogProto;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public class EventLog {
    private SkipblockId genesis;
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
    }

    private byte[] init(Roster roster, Darc ownerDarc) throws CothorityCommunicationException  {
        EventLogProto.InitRequest.Builder req = EventLogProto.InitRequest.newBuilder();
        req.setRoster(roster.toProto());
        req.setOwner(ownerDarc.toProto());

        ByteString msg = roster.sendMessage("EventLog/InitRequest", req.build());

        try {
            EventLogProto.InitResponse reply = EventLogProto.InitResponse.parseFrom(msg);
            return reply.getId().toByteArray();
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    public void log(Event event) {
    }

    /*
    private List<TransactionProto.ClientTransaction> makeTx(List<Event> events, DarcId darcID, List<Signer> signers) {

    }
    */
}
