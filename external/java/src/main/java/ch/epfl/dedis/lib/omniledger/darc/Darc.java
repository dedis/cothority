package ch.epfl.dedis.lib.omniledger.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.*;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/**
 * Darc stands for distributed access right control. It provides a powerful access control policy that supports logical
 * expressions, delegation of rights, offline verification and so on. Please refer to
 * https://github.com/dedis/cothority/omniledger/README.md#darc for more information.
 */
public class Darc {
    private int version;
    private byte[] description;
    private DarcId baseID;
    private DarcId prevID;
    private String evolveName;
    private String signName;
    private Map<String, byte[]> rules;
    private List<Signature> signatures;
    private List<Darc> verificationDarcs;

    /**
     * The Darc constructor.
     * @param rules The initial set of rules, consider using initRules to create them.
     * @param desc The description.
     */
    public Darc(List<Identity> owners, List<Identity> signers, String  evolveName, String signName, byte[] desc) {
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
        this.evolveName = evolveName;
        this.signName = signName;
        this.rules = Darc.initRules(owners, signers);
        this.signatures = new ArrayList<>();
        this.verificationDarcs = new ArrayList<>();
    }

    /**
     * Creates the protobuf representation of the darc.
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
        b.setEvolvename(this.evolveName);
        b.setSignname(this.signName);
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
            digest.update(Darc.intToArr8(this.version));
            digest.update(this.description);
            if (this.baseID != null) {
                digest.update(this.baseID.getId());
            }
            digest.update(this.prevID.getId());
            digest.update(this.evolveName.getBytes());
            digest.update(this.signName.getBytes());
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
     * Gets the base-ID of the darc, i.e. the ID before any evolution.
     * @return base-ID
     * @throws CothorityCryptoException
     */
    public DarcId getBaseId() throws CothorityCryptoException {
        if (this.version == 0 ) {
            return this.getId();
        }
        return this.baseID;
    }

    /**
     * This method sets the sign expression in the rules. The old expression will be replaced.
     * @param expr The expression to set.
     */
    public void setSignExpr(byte[] expr) {
        this.rules.put(this.signName, expr);
    }

    /**
     * This method sets the evolve expression in the rules. The old expression will be replaced.
     * @param expr The expression to set.
     */
    public void setEvolveExpr(byte[] expr) {
        this.rules.put(this.evolveName, expr);
    }

    /**
     * This method sets a given action to the given expression. If the action conflicts with the evolve action name or
     * the sign action name then the function does nothing. If the action exists, then its expression will be
     * overwritten.
     * @param action The action that we are setting.
     * @param expr The expression that corresponds to the action.
     */
    public void setExpr(String action, byte[] expr) {
        if (action.equals(this.evolveName) || action.equals(this.signName)) {
            return;
        }
        this.rules.put(action, expr);
    }

    /**
     * This methods gets the sign expression. If the return value is null then the darc was not initialised correctly.
     * @return The expression.
     */
    public byte[] getSignExpr() {
        return this.rules.get(this.signName);
    }

    /**
     * This methods gets the evolve expression. If the return value is null then the darc was not initialised correctly.
     * @return The expression.
     */
    public byte[] getEvolveExpr() {
        return this.rules.get(this.evolveName);
    }

    /**
     * This is a convenience function that initialise a set of rules with the default actions "_evolve" and "_sign".
     * Signers are joined with logical-Or, owners are joined with logical-AND. If other expressions are needed, please
     * set the rules manually.
     * @param owners A list of owners.
     * @param signers A list of signers.
     * @return The action-expression mapping, also known as the rule.
     */
    private static Map<String, byte[]> initRules(List<Identity> owners, List<Identity> signers)  {
        Map<String, byte[]> rs = new HashMap<>();
        List<String> ownerIDs = owners.stream().map(Identity::toString).collect(Collectors.toList());
        rs.put("_evolve", String.join(" & ", ownerIDs).getBytes());

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
}
