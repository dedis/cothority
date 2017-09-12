import com.moandjiezana.toml.Toml;

import java.util.List;
import java.util.stream.Collectors;

public class Roster {
    public List<ServerIdentity> Nodes;

    public Roster(String group_toml){
        Toml toml = new Toml().read(group_toml);
        List<Toml> servers = toml.getTables("servers");
        this.Nodes = servers.stream().map(server -> new ServerIdentity(server)).collect(Collectors.toList());
    }
}
