package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Crypto;
import ch.epfl.dedis.ocs.Account;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

import java.util.ArrayList;
import java.util.List;

/**
 * dedis/lib
 * Darc.java
 * Purpose: Distributed Access Rights Control - an experimental idea how to get user management
 * decentralized.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */

public class Darc {
    public byte[] id;
    public List<DarcLink> accounts;
    public List<Crypto.Point> points;
    public int version;
    public DarcSig signature;

    public Darc() {
        this(Crypto.uuid4());
    }

    public Darc(byte[] id) {
        this.id = id;
        accounts = new ArrayList<>();
        points = new ArrayList<>();
    }

    public Darc(DarcProto.Darc d) {
        id = d.getId().toByteArray();
        accounts = new ArrayList<>();
        d.getAccountsList().forEach(al -> accounts.add(new DarcLink(al)));
        points = new ArrayList<>();
        d.getPublicKeysList().forEach(pub -> points.add(new Crypto.Point(pub)));
        version = d.getVersion();
        signature = new DarcSig(d.getSignature());
    }

    public DarcProto.Darc getProto() {
        DarcProto.Darc.Builder d = DarcProto.Darc.newBuilder();
        d.setId(ByteString.copyFrom(id));
        accounts.forEach(a -> d.addAccounts(a.getProto()));
        points.forEach(p -> d.addPublicKeys(p.toProto()));
        d.setVersion(version);
        if (signature != null) {
            d.setSignature(signature.getProto());
        }
        return d.build();
    }

    static public class DarcLink {
        public byte[] id;
        public int rights;
        public int threshold;

        public DarcLink(Account a) {
            id = a.ID;
            rights = a.Access;
            threshold = 1;
        }

        public DarcLink(DarcProto.DarcLink dl) {
            id = dl.getId().toByteArray();
            rights = dl.getRights();
            threshold = dl.getThreshold();
        }

        public DarcProto.DarcLink getProto() {
            DarcProto.DarcLink.Builder dl = DarcProto.DarcLink.newBuilder();
            dl.setId(ByteString.copyFrom(id));
            dl.setRights(rights);
            dl.setThreshold(threshold);

            return dl.build();
        }
    }

    static public class DarcSig {
        public byte[] id;
        public int version;
        public byte[] signature;

        public DarcSig(DarcProto.DarcSig ds) {
            id = ds.getId().toByteArray();
            version = ds.getVersion();
            signature = ds.getSignature().toByteArray();
        }

        public DarcProto.DarcSig getProto() {
            DarcProto.DarcSig.Builder ds = DarcProto.DarcSig.newBuilder();
            ds.setId(ByteString.copyFrom(id));
            ds.setVersion(version);
            ds.setSignature(ByteString.copyFrom(signature));

            return ds.build();
        }
    }
}
