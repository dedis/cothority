package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.*;
import java.util.stream.Collectors;
import java.util.stream.Stream;

public class Darc {
    private int version;
    private byte[] description;
    private DarcId baseID;
    private Map<String, byte[]> rules;
    private List<Darc> path;
    private byte[] pathDigest;
    public List<Signature> signatures;
    private final Logger logger = LoggerFactory.getLogger(Darc.class);

    public static Map<String, byte[]> initRules(List<Identity> owners, List<Identity> signers)  {
        Map<String, byte[]> rs = new HashMap<>();
        List<String> ownerIDs = owners.stream().map((owner) -> owner.toString()).collect(Collectors.toList());
        rs.put("_evolve", String.join(" & ", ownerIDs).getBytes());

        List<String> signerIDs = signers.stream().map((signer) -> signer.toString()).collect(Collectors.toList());
        rs.put("_sign", String.join(" | ", signerIDs).getBytes());
        return rs;
    }

    public Darc(Map<String, byte[]> rules, byte[] desc) {
        this.version = 0;
        this.description = desc;
        this.baseID = null;
        this.rules = rules;
        this.path = new ArrayList<>();
        this.signatures = new ArrayList<>();

        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(new byte[0]);
            this.pathDigest = digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * Creates a protobuf representation of the darc.
     *
     * @return the protobuf representation of the darc.
     */
    public DarcProto.Darc toProto() {
        DarcProto.Darc.Builder b = DarcProto.Darc.newBuilder();
        b.setVersion(this.version);
        b.setDescription(ByteString.copyFrom(this.description));
        for (Map.Entry<String, byte[]> entry : this.rules.entrySet()) {
            b.putRules(entry.getKey(), ByteString.copyFrom(entry.getValue()));
        }
        this.path.forEach((d) -> b.addPath(d.toProto()));
        b.setPathdigest(ByteString.copyFrom(this.pathDigest));
        this.signatures.forEach((s) -> b.addSignatures(s.toProto()));
        if (this.baseID != null) {
            b.setBaseid(ByteString.copyFrom(this.baseID.getId()));
        }
        return b.build();
    }

    /**
     * Calculate the getId of the darc by calculating the sha-256 of the invariant
     * parts which excludes the delegation-signature.
     *
     * @return sha256
     */
    public DarcId getId() throws CothorityCryptoException {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(Darc.intToArr(this.version));
            digest.update(this.description);
            if (this.baseID != null) {
                digest.update(this.baseID.getId());
            }
            digest.update(this.pathDigest);
            this.sortedAction().forEach((k) -> {
                byte[] expr = this.rules.get(k);
                digest.update(k.getBytes());
                digest.update(expr);
            });
            return new DarcId(digest.digest());
        } catch (NoSuchAlgorithmException e) {
            // TODO we should throw exceptions
            throw new RuntimeException(e);
        }
    }

    public DarcId getBaseId() throws CothorityCryptoException {
        if (this.version == 0 ) {
            return this.getId();
        }
        return this.baseID;
    }

    private Stream<String> sortedAction() {
        return this.rules.keySet().stream().sorted();
    }

    private static byte[] intToArr(int x) {
        ByteBuffer b = ByteBuffer.allocate(8);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putInt(x);
        return b.array();
    }
}
