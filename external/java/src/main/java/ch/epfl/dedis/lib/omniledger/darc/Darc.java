package ch.epfl.dedis.lib.omniledger.darc;

import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/**
 * Darc stands for distributed access right control. It provides a powerful access control policy that supports logical
 * expressions, delegation of rights, offline verification and so on. Please refer to
 * https://github.com/dedis/cothority/omniledger/README.md#darc for more information.
 */
public class Darc {
    private long version;
    private byte[] description;
    private DarcId baseID;
    private DarcId prevID;
    private Map<String, byte[]> rules;
    private List<Signature> signatures;
    private List<Darc> verificationDarcs;

    private final static Logger logger = LoggerFactory.getLogger(Darc.class);

    /**
     * The Darc constructor.
     *
     * @param rules The initial set of rules, consider using initRules to create them.
     * @param desc  The description.
     */
    public Darc(Map<String, byte[]> rules, byte[] desc) {
        this.version = 0;
        this.description = desc;
        this.baseID = null;
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(new byte[0]);
            this.prevID = new DarcId(digest.digest());
        } catch (NoSuchAlgorithmException | CothorityCryptoException e) {
            // NoSuchAlgorithmException or CothorityCryptoException should never happen because SHA-256 exists and the
            // digest of it has the right length (32 bytes).
            throw new RuntimeException(e);
        }
        this.rules = rules;
        this.signatures = new ArrayList<>();
        this.verificationDarcs = new ArrayList<>();

    }

    /**
     * Convenience constructor
     *
     * @param owners  a list of owners that are allowed to evolve the darc
     * @param signers a list of signers on behalf of that darc
     * @param desc    free form description of the darc
     */
    public Darc(List<Identity> owners, List<Identity> signers, byte[] desc) {
        this(initRules(owners, signers), desc);
    }

    public Darc(DarcProto.Darc proto) throws CothorityCryptoException {
        version = proto.getVersion();
        description = proto.getDescription().toByteArray();
        if (version > 0) {
            logger.info("setting baseID");
            baseID = new DarcId(proto.getBaseid());
        }
        prevID = new DarcId(proto.getPrevid());
        rules = new HashMap<>();
        Map<String, ByteString> protoRules = proto.getRulesMap();
        for (String key : protoRules.keySet()) {
            rules.put(key, protoRules.get(key).toByteArray());
        }
        signatures = new ArrayList<>();
        for (DarcProto.Signature sig : proto.getSignaturesList()) {
            signatures.add(new Signature(sig));
        }
        logger.info("BaseID is {}", baseID);
    }

    public Darc(byte[] buf) throws InvalidProtocolBufferException, CothorityCryptoException {
        this(DarcProto.Darc.parseFrom(buf));
    }

    /**
     * Sets a rule to be the action/expression pair. This will overwrite an
     * existing rule or create a new one.
     *
     * @param action
     * @param expression
     */
    public void setRule(String action, byte[] expression) {
        rules.put(action, expression);
    }

    /**
     * Updates the version of the darc and clears any eventual signatures from previous
     * evolutions.
     */
    public void increaseVersion() throws CothorityCryptoException {
        version++;
        signatures = new ArrayList<>();
        verificationDarcs = new ArrayList<>();
    }

    /**
     * Creates the protobuf representation of the darc.
     *
     * @return The protobuf representation.
     */
    public DarcProto.Darc toProto() {
        DarcProto.Darc.Builder b = DarcProto.Darc.newBuilder();
        b.setVersion(this.version);
        b.setDescription(ByteString.copyFrom(this.description));
        if (this.baseID != null) {
            b.setBaseid(ByteString.copyFrom(this.baseID.getId()));
        }
        b.setPrevid(ByteString.copyFrom(this.prevID.getId()));
        for (Map.Entry<String, byte[]> entry : this.rules.entrySet()) {
            b.putRules(entry.getKey(), ByteString.copyFrom(entry.getValue()));
        }
        this.verificationDarcs.forEach((d) -> b.addVerificationdarcs(d.toProto()));
        this.signatures.forEach((s) -> b.addSignatures(s.toProto()));
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
            digest.update(Darc.longToArr8(this.version));
            digest.update(this.description);
            if (this.baseID != null) {
                digest.update(this.baseID.getId());
            }
            digest.update(this.prevID.getId());
            this.sortedAction().forEach((k) -> {
                byte[] expr = this.rules.get(k);
                digest.update(k.getBytes());
                digest.update(expr);
            });
            return new DarcId(digest.digest());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * @param id the id of the previous darc.
     */
    public void setPrevId(DarcId id) {
        prevID = id;
    }

    /**
     * @param id the base-id to set
     */
    public void setBaseId(DarcId id) {
        baseID = id;
    }

    /**
     * @param d the previous darc
     * @throws CothorityCryptoException
     */
    public void setPrevId(Darc d) throws CothorityCryptoException {
        setPrevId(d.getId());
    }

    /**
     * Gets the base-ID of the darc, i.e. the ID before any evolution.
     *
     * @return base-ID
     * @throws CothorityCryptoException
     */
    public DarcId getBaseId() throws CothorityCryptoException {
        if (version == 0) {
            return getId();
        }
        return baseID;
    }

    public DarcId getPrevID() {
        return prevID;
    }

    /**
     * @return the current version.
     */
    public long getVersion() {
        return version;
    }

    /**
     * @return a copy of the darc with the same version number.
     * @throws CothorityCryptoException
     */
    public Darc copy() throws CothorityCryptoException {
        Map<String, byte[]> rs = new HashMap<>();
        for (String k : rules.keySet()) {
            rs.put(k, rules.get(k));
        }
        Darc c = new Darc(rs, description.clone());
        c.version = version;
        return c;
    }

    /**
     * @return a copy of the darc with the next version number and prevId and baseId set up.
     * @throws CothorityCryptoException
     */
    public Darc copyEvolve() throws CothorityCryptoException {
        Map<String, byte[]> rs = new HashMap<>();
        for (String k : rules.keySet()) {
            rs.put(k, rules.get(k));
        }
        Darc c = new Darc(rs, description.clone());
        c.version = version + 1;
        c.prevID = getId();
        c.baseID = getBaseId();
        return c;
    }

    public String toString() {
        try {
            String base = Hex.printHexBinary(getBaseId().getId());
            if (baseID != null) {
                base = String.format("stored: %s", Hex.printHexBinary(baseID.getId()));
            }
            String ret = String.format("Base: %s\nId: %s\nPrevId: %s\nVersion: %d\nRules:",
                    base,
                    Hex.printHexBinary(getId().getId()),
                    Hex.printHexBinary(getPrevID().getId()),
                    version);
            for (String r : rules.keySet()) {
                ret += String.format("\n%s - %s", r, Hex.printHexBinary(rules.get(r)));
            }
            ret += String.format("\nDescription: %s", Hex.printHexBinary(description));
            return ret;
        } catch (CothorityException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * This is a convenience function that initialise a set of rules with the default actions "_evolve" and "_sign".
     * Signers are joined with logical-Or, owners are joined with logical-AND. If other expressions are needed, please
     * set the rules manually.
     *
     * @param owners  A list of owners.
     * @param signers A list of signers.
     * @return The action-expression mapping, also known as the rule.
     */
    public static Map<String, byte[]> initRules(List<Identity> owners, List<Identity> signers) {
        Map<String, byte[]> rs = new HashMap<>();
        List<String> ownerIDs = owners.stream().map(Identity::toString).collect(Collectors.toList());
        rs.put("invoke:evolve", String.join(" & ", ownerIDs).getBytes());

        List<String> signerIDs = signers.stream().map(Identity::toString).collect(Collectors.toList());
        rs.put("_sign", String.join(" | ", signerIDs).getBytes());
        return rs;
    }

    private Stream<String> sortedAction() {
        return this.rules.keySet().stream().sorted();
    }

    private static byte[] intToArr8(int x) {
        ByteBuffer b = ByteBuffer.allocate(8);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putInt(x);
        return b.array();
    }

    private static byte[] longToArr8(long x) {
        ByteBuffer b = ByteBuffer.allocate(8);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putLong(x);
        return b.array();
    }
}
