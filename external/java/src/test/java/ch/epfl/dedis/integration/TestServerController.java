package ch.epfl.dedis.integration;

import ch.epfl.dedis.byzgen.OcsFactory;
import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;

import javax.annotation.Nonnull;
import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.util.List;
import java.util.stream.Collectors;

public abstract class TestServerController {
    protected static final String CONODE_PUB_1 = "d829a0790ffa8799e4bbd1bee8da0507c9166b665660baba72dd8610fca27cc1";
    protected static final String CONODE_PUB_2 = "d750a30daa44713d1a4b44ca4ef31142b3b53c0c36a558c0d610cc4108bb4ecb";
    protected static final String CONODE_PUB_3 = "7f47f33084c3ecc233f8b05b8f408bbd1c2e4a129aae126f92becacc73576bc7";
    protected static final String CONODE_PUB_4 = "8b25f8ac70b85b2e9aa7faf65507d4f7555af1c872240305117b7659b1e58a1e";

    public abstract int countRunningConodes() throws IOException, InterruptedException;

    public abstract void startConode(int nodeNumber) throws InterruptedException, IOException;

    public abstract void killConode(int nodeNumber) throws IOException, InterruptedException;

    public abstract List<OcsFactory.ConodeAddress> getConodes();

    public Roster getRoster() {
        return new Roster(getConodes().stream()
                .map(conodeAddress -> new ServerIdentity(conodeAddress.getAddress(), conodeAddress.getPublicKey()))
                .collect(Collectors.toList()));
    }

    public OcsFactory.ConodeAddress getMasterConode() {
        return getConodes().get(0);
    }

    @Nonnull
    public static URI buildURI(String str) {
        try {
            return new URI(str);
        } catch (URISyntaxException e) {
            throw new IllegalStateException("Unable to setup test instance", e);
        }
    }
}
