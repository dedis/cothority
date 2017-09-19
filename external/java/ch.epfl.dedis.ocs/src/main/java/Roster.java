import com.google.protobuf.ByteString;
import com.moandjiezana.toml.Toml;
import net.i2p.crypto.eddsa.math.GroupElement;
import proto.RosterProto;

import java.security.PublicKey;
import java.util.List;
import java.util.stream.Collectors;

public class Roster {
    public List<ServerIdentity> Nodes;
    public Crypto.Point Aggregate;

    public Roster(String group_toml){
        Toml toml = new Toml().read(group_toml);
        List<Toml> servers = toml.getTables("servers");
        Nodes = servers.stream().map(server -> new ServerIdentity(server)).collect(Collectors.toList());
        Aggregate = Nodes.get(0).Public;
        for (int i = 1; i < Nodes.size(); i++){
            Aggregate = Aggregate.add(Nodes.get(i).Public);
        }
    }

    public RosterProto.Roster getProto() throws Exception{
        RosterProto.Roster.Builder r = RosterProto.Roster.newBuilder();
        r.setId(ByteString.copyFrom(Crypto.uuid4()));
        Nodes.forEach(n -> r.addList(n.getProto()));
        r.setAggregate(Aggregate.toProto());

        return r.build();
    }
}
