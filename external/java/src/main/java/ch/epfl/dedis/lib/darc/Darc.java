package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

public class Darc {
    public List<Identity> owners;
    private List<Identity> users;
    private byte[] data;
    private int version;
    private DarcId baseid;
    private DarcSignature signature;
    private final Logger logger = LoggerFactory.getLogger(Darc.class);

    /**
     * Initialize a darc by giving an owner that is allowed to evolve the darc
     * and a list of users allowed to sign actions on behalf of that darc.
     * The data fieldElement can be used for any application-specific usage and will
     * not be interpreted by darc or onchain-secrets.
     *
     * @param owners defines who will be allowed to evolve that darc
     * @param users  defines who can sign on behalf of that darc
     */
    public Darc(List<Identity> owners, List<Identity> users, byte[] data) throws CothorityCryptoException {
        this();
        version = 0;
        if (owners != null) {
            this.owners = new ArrayList<>(owners);
        }
        if (users != null) {
            this.users = new ArrayList<>(users);
        }
        setDataAndInitBase(data);
    }

    /**
     * Initialize a darc by giving an owner that is allowed to evolve the darc
     * and a list of users allowed to sign actions on behalf of that darc.
     * The data fieldElement can be used for any application-specific usage and will
     * not be interpreted by darc or onchain-secrets.
     *
     * @param owner defines who will be allowed to evolve that darc
     * @param users defines who can sign on behalf of that darc
     */
    public Darc(Identity owner, List<Identity> users, byte[] data) throws CothorityException {
        this();
        version = 0;
        owners.add(owner);
        if (users != null) {
            this.users = new ArrayList<>(users);
        }
        setDataAndInitBase(data);
    }

    /**
     * Overloaded function for convenience. Directly creates the Identities from
     * the signers.
     *
     * @param owner
     * @param users
     * @param data
     */
    public Darc(Signer owner, List<Signer> users, byte[] data) throws CothorityCryptoException {
        this();
        version = 0;
        owners.add(IdentityFactory.New(owner));
        if (users != null) {
            for (Signer s : users) {
                this.users.add(IdentityFactory.New(s));
            }
        }
        setDataAndInitBase(data);
    }

    /**
     * Overloaded function to create an empty darc.
     */
    public Darc() {
        owners = new ArrayList<>();
        users = new ArrayList<>();
    }

    /**
     * helper function to set data and initialise the baseid
     *
     * @param data what should be stored in the data-field
     * @throws CothorityCryptoException
     */
    private void setDataAndInitBase(byte[] data) throws CothorityCryptoException {
        this.data = data;
        SecureRandom random = new SecureRandom();
        byte bytes[] = new byte[DarcId.length];
        random.nextBytes(bytes);
        this.baseid = new DarcId(bytes);
    }

    /**
     * Returns a darc from a protobuf representation.
     *
     * @param proto
     */
    public Darc(DarcProto.Darc proto) throws CothorityCryptoException {
        this();
        for (DarcProto.Identity owner : proto.getOwnersList()) {
            owners.add(IdentityFactory.New(owner));
        }
        for (DarcProto.Identity user : proto.getUsersList()) {
            users.add(IdentityFactory.New(user));
        }
        version = proto.getVersion();
        if (proto.hasDescription()) {
            data = proto.getDescription().toByteArray();
        }
        if (proto.hasSignature()) {
            signature = new DarcSignature(proto.getSignature());
        }
        if (proto.hasBaseid()) {
            baseid = new DarcId(proto.getBaseid().toByteArray());
        }
    }

    /**
     * Creates a copy of the current darc and increases the version-number
     * by 1. Once the new darc is configured, setEvolution has to be called
     * last on the new darc.
     *
     * @return new darc
     */
    public Darc copy() throws CothorityCryptoException {
        Darc d = new Darc(owners, users, data);
        d.version = version;
        d.baseid = baseid;
        return d;
    }

    /**
     * Calculate the getId of the darc by calculating the sha-256 of the invariant
     * parts which excludes the delegation-signature.
     *
     * @return sha256
     */
    public DarcId getId() throws CothorityCryptoException {
        Darc c = copy();
        DarcProto.Darc proto = c.toProto();
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(proto.toByteArray());
            return new DarcId(hash);
        } catch (NoSuchAlgorithmException e) {
            return null;
        }
    }

    /**
     * To evolve a darc, the latest valid darc needs to sign the new darc.
     * Only if one of the previous owners signs off on the new darc will it be
     * valid and accepted to sign on behalf of the old darc. The path can be nil
     * unless if the previousOwner is an Ed25519Signer and found directly in the
     * previous darc.
     *
     * @param previous
     * @param path
     * @param previousOwner
     */
    public void setEvolution(Darc previous, SignaturePath path, Signer previousOwner) throws CothorityCryptoException {
        version = previous.version + 1;
        if (path == null) {
            path = new SignaturePath(previous, previousOwner, SignaturePath.OWNER);
        }
        boolean found = false;
        Identity signerId = IdentityFactory.New(previousOwner);
        for (Identity id: path.getDarcs().get(path.getDarcs().size()-1).owners){
            if (id.equals(signerId)){
                found = true;
                break;
            }
        }
        if (!found){
            throw new CothorityCryptoException("Wrong path: signer is not in last darc.");
        }
        baseid = previous.getBaseId();
        signature = new DarcSignature(getId().getId(), path, previousOwner);
        logger.debug("Signature is: " + signature.toProto().toString());
    }

    /**
     * Returns the id of the darc with version == 0. If it's the darc with version
     * 0, then it will return its own getId.
     *
     * @return id of first darc
     */
    public DarcId getBaseId() throws CothorityCryptoException {
        if (version > 0) {
            return baseid;
        }
        return getId();
    }

    /**
     * Returns true if the current darc has correctly been evolved from the previous darc.
     *
     * @param previous
     * @return
     */
    public boolean verifyEvolution(Darc previous) throws CothorityCryptoException {
        if (signature == null) {
            return false;
        }
        return signature.verify(getId().getId(), previous);
    }

    /**
     * Creates a protobuf representation of the darc.
     *
     * @return the protobuf representation of the darc.
     */
    public DarcProto.Darc toProto() {
        DarcProto.Darc.Builder b = DarcProto.Darc.newBuilder();
        for (Identity i : owners) {
            b.addOwners(i.toProto());
        }
        for (Identity i : users) {
            b.addUsers(i.toProto());
        }
        if (signature != null) {
            b.setSignature(signature.toProto());
        }
        b.setVersion(version);
        if (data != null) {
            b.setDescription(ByteString.copyFrom(data));
        }
        if (baseid != null) {
            b.setBaseid(ByteString.copyFrom(baseid.getId()));
        }
        return b.build();
    }

    /**
     * Adds a user to the list of allowed signers.
     *
     * @param identity
     */
    public void addUser(Identity identity) {
        users.add(identity);
    }

    /**
     * Adds a user to the list of allowed signers.
     *
     * @param darc
     */
    public void addUser(Darc darc) throws CothorityCryptoException {
        addUser(IdentityFactory.New(darc));
    }

    /**
     * Adds a user to the list of allowed signers.
     *
     * @param signer
     */
    public void addUser(Signer signer) throws CothorityCryptoException {
        addUser(IdentityFactory.New(signer));
    }

    /**
     * Adds a owner to the list of allowed signers.
     *
     * @param identity
     */
    public void addOwner(Identity identity) {
        owners.add(identity);
    }

    /**
     * Adds an owner to the list of allowed signers.
     *
     * @param darc
     */
    public void addOwner(Darc darc) throws CothorityCryptoException {
        addOwner(IdentityFactory.New(darc));
    }

    /**
     * Adds a owner to the list of allowed signers.
     *
     * @param signer
     */
    public void addOwner(Signer signer) throws CothorityCryptoException {
        addOwner(IdentityFactory.New(signer));
    }

    /**
     * Increments the version of this Darc by 1.
     */
    public void incVersion() {
        version++;
    }

    /**
     * Returns the current version
     *
     * @return
     */
    public int getVersion() {
        return version;
    }

    /**
     * Retrun copy of current owners of DARC
     * @return list of owners
     */
    public List<Identity> getOwners() {
        return new ArrayList<>(owners);
    }

    /**
     * Return copy of current users of DARC (users/dacs who can execute this DARC)
     * @return list of users
     */
    public List<Identity> getUsers() {
        return new ArrayList<>(users);
    }

    public byte[] getData() {
        if (data==null) {
            return null;
        }
        return Arrays.copyOf(data, data.length);
    }

    public String toString() {
        try {
            String ret = String.format("getId: %s\n", getId().toString());
            if (baseid != null) {
                ret += String.format("BaseID: %s\n", baseid.toString());
            }
            for (Identity i : owners) {
                ret += String.format("owner: %s\n", i.toString());
            }
            for (Identity i : users) {
                ret += String.format("user: %s\n", i.toString());
            }
            return ret;
        } catch (CothorityCryptoException e) {
            return "Error when creating string: " + e.toString();
        }
    }

    public boolean equals(Darc d) {
        try {
            return this.getId().equals(d.getId());
        } catch (CothorityCryptoException e) {
            return false;
        }
    }
}
