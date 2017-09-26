package ch.epfl.dedis.ocs;

import java.net.URI;
import java.net.URISyntaxException;

public class LocalRosters {
    public static final String CONODE_PUB_1 = "2CmgeQ/6h5nku9G+6NoFB8kWa2ZWYLq6ct2GEPyifME=";
    public static final String CONODE_PUB_2 = "11CjDapEcT0aS0TKTvMRQrO1PAw2pVjA1hDMQQi7Tss=";
    public static final String CONODE_PUB_3 = "f0fzMITD7MIz+LBbj0CLvRwuShKarhJvkr7KzHNXa8c=";

    public static final URI CONODE_1 = buildURI("tcp://127.0.0.1:7002");
    public static final URI CONODE_2 = buildURI("tcp://127.0.0.1:7004");
    public static final URI CONODE_3 = buildURI("tcp://127.0.0.1:7006");

    public static String[] ids = {"482fb9cfc2b55ab68c5f811c1d47b9e1",
            "4266ca2b721557b4977e8d4b691f88c1",
            "5fa48b4050ed5bc59452b49c08f66368"};

    public static String aggregate = "c482b8c0eef4a200227c21f09c30a837d2c9b2b2e16972262dafac46a4f2c572";

    static URI buildURI(String uri) {
        try {
            return new URI(uri);
        } catch (URISyntaxException e) {
            return null;
        }
    }

    public static String groupToml = "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7002\"\n" +
            "  Point = \"2CmgeQ/6h5nku9G+6NoFB8kWa2ZWYLq6ct2GEPyifME=\"\n" +
            "  Description = \"Conode_1\"\n" +
            "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7004\"\n" +
            "  Point = \"11CjDapEcT0aS0TKTvMRQrO1PAw2pVjA1hDMQQi7Tss=\"\n" +
            "  Description = \"Conode_2\"\n" +
            "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7006\"\n" +
            "  Point = \"f0fzMITD7MIz+LBbj0CLvRwuShKarhJvkr7KzHNXa8c=\"\n" +
            "  Description = \"Conode_3\"\n";

    public static String firstToml = "[[servers]]\n" +
            "  Address = \"tcp://127.0.0.1:7002\"\n" +
            "  Point = \"2CmgeQ/6h5nku9G+6NoFB8kWa2ZWYLq6ct2GEPyifME=\"\n" +
            "  Description = \"Conode_1\"";
}
