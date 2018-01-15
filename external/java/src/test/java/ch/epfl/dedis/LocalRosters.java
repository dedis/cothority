package ch.epfl.dedis;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import com.moandjiezana.toml.Toml;

import java.net.URI;
import java.net.URISyntaxException;
import java.util.ArrayList;
import java.util.List;

public class LocalRosters {
    public static final String CONODE_PUB_1 = "d829a0790ffa8799e4bbd1bee8da0507c9166b665660baba72dd8610fca27cc1";
    public static final String CONODE_PUB_2 = "d750a30daa44713d1a4b44ca4ef31142b3b53c0c36a558c0d610cc4108bb4ecb";
    public static final String CONODE_PUB_3 = "7f47f33084c3ecc233f8b05b8f408bbd1c2e4a129aae126f92becacc73576bc7";
    public static final String CONODE_PUB_4 = "8b25f8ac70b85b2e9aa7faf65507d4f7555af1c872240305117b7659b1e58a1e";

    public static final URI CONODE_1 = buildURI("tcp://127.0.0.1:7002");
    public static final URI CONODE_2 = buildURI("tcp://127.0.0.1:7004");
    public static final URI CONODE_3 = buildURI("tcp://127.0.0.1:7006");
    public static final URI CONODE_4 = buildURI("tcp://127.0.0.1:7008");

    public static String aggregate = "07e360dd42042ab75c1c639bc97827285083994d2c181aaf586032561a310d9a";

    static URI buildURI(String uri) {
        try {
            return new URI(uri);
        } catch (URISyntaxException e) {
            return null;
        }
    }

    public static String groupToml = "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7002\"\n" +
            "  Public = \"d829a0790ffa8799e4bbd1bee8da0507c9166b665660baba72dd8610fca27cc1\"\n" +
            "  Description = \"Conode_1\"\n" +
            "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7004\"\n" +
            "  Public = \"d750a30daa44713d1a4b44ca4ef31142b3b53c0c36a558c0d610cc4108bb4ecb\"\n" +
            "  Description = \"Conode_2\"\n" +
            "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7006\"\n" +
            "  Public = \"7f47f33084c3ecc233f8b05b8f408bbd1c2e4a129aae126f92becacc73576bc7\"\n" +
            "  Description = \"Conode_3\"\n";

    public static String firstToml = "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7002\"\n" +
            "  Public = \"d829a0790ffa8799e4bbd1bee8da0507c9166b665660baba72dd8610fca27cc1\"\n" +
            "  Description = \"Conode_1\"\n";

    public static Roster FromToml(String groupToml) {
        Toml toml = new Toml().read(groupToml);
        List<ServerIdentity> cothority = new ArrayList<>();
        List<Toml> servers = toml.getTables("servers");

        for (Toml s : servers) {
            try {
                cothority.add(new ServerIdentity(s));
            } catch (URISyntaxException e) {
            }
        }
        return new Roster(cothority);
    }
}
