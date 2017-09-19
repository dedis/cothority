package ch.epfl.dedis.lib;

import ch.epfl.dedis.proto.OCSProto;
import ch.epfl.dedis.proto.RosterProto;
import com.google.protobuf.ByteString;
import com.moandjiezana.toml.Toml;

import javax.xml.bind.DatatypeConverter;
import java.util.List;
import java.util.stream.Collectors;

/**
 * dedis/lib
 * Roster.java
 * Purpose: A list of ServerIdentities make up a roster that can be used as a temporary
 * cothority.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */

public class Roster {
    public List<ServerIdentity> Nodes;
    public Crypto.Point Aggregate;

    public Roster(String group_toml) {
        Toml toml = new Toml().read(group_toml);
        List<Toml> servers = toml.getTables("servers");
        Nodes = servers.stream().map(server -> new ServerIdentity(server)).collect(Collectors.toList());
        Aggregate = Nodes.get(0).Public;
        for (int i = 1; i < Nodes.size(); i++) {
            Aggregate = Aggregate.add(Nodes.get(i).Public);
        }
    }

    public RosterProto.Roster getProto() throws Exception {
        RosterProto.Roster.Builder r = RosterProto.Roster.newBuilder();
        r.setId(ByteString.copyFrom(Crypto.uuid4()));
        Nodes.forEach(n -> r.addList(n.getProto()));
        r.setAggregate(Aggregate.toProto());

        return r.build();
    }

    public ByteString SendMessage(String path, com.google.protobuf.GeneratedMessageV3 proto) throws CothorityError{
        // TODO - fetch a random node.
        return ByteString.copyFrom(Nodes.get(0).SendMessage(path, proto.toByteArray()));
    }
}
