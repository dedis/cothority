package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;
import com.sun.xml.internal.ws.util.ByteArrayBuffer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.xml.bind.DatatypeConverter;
import java.io.IOException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

public class Darc {
    public List<Identity> owners;
    private List<Identity> users;
    private byte[] data;
    private int version;
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
    public Darc(List<Identity> owners, List<Identity> users, byte[] data) {
        this();
        version = 0;
        if (owners != null) {
            this.owners = new ArrayList<>(owners);
        }
        if (users != null) {
            this.users = new ArrayList<>(users);
        }
        this.data = data;
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
    public Darc(Identity owner, List<Identity> users, byte[] data) {
        this();
        version = 0;
        owners.add(owner);
        if (users != null) {
            this.users = new ArrayList<>(users);
        }
        this.data = data;
    }

    /**
     * Overloaded function for convenience. Directly creates the Identities from
     * the signers.
     *
     * @param owner
     * @param users
     * @param data
     */
    public Darc(Signer owner, Signer[] users, byte[] data) throws Exception {
        this();
        version = 0;
        owners.add(IdentityFactory.New(owner));
        if (users != null) {
            this.users = new ArrayList<>();
            for (Signer s : users) {
                this.users.add(IdentityFactory.New(s));
            }
        }
        this.data = data;
    }

    /**
     * Overloaded function to create an empty darc.
     */
    public Darc() {
        owners = new ArrayList<>();
        users = new ArrayList<>();
    }

    /**
     * Returns a darc from a protobuf representation.
     *
     * @param proto
     */
    public Darc(DarcProto.Darc proto) throws Exception {
        this();
        for (DarcProto.Identity owner : proto.getOwnersList()) {
            owners.add(IdentityFactory.New(owner));
        }
        for (DarcProto.Identity user : proto.getUsersList()) {
            owners.add(IdentityFactory.New(user));
        }
        version = proto.getVersion();
        if (proto.hasDescription()){
            data = proto.getDescription().toByteArray();
        }
        if (proto.hasSignature()){
            signature = new DarcSignature(proto.getSignature());
        }
    }

    /**
     * Creates a copy of the current darc and increases the version-number
     * by 1. Once the new darc is configured, SetEvolution has to be called
     * last on the new darc.
     *
     * @return new darc
     */
    public Darc Copy() {
        Darc d = new Darc(owners, users, data);
        d.version = version;
        return d;
    }

    /**
     * Calculate the ID of the darc by calculating the sha-256 of the invariant
     * parts which excludes the delegation-signature.
     *
     * @return sha256
     */
    public byte[] ID() {
        Darc c = Copy();
        DarcProto.Darc proto = c.ToProto();
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(proto.toByteArray());
            return hash;
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
    public void SetEvolution(Darc previous, SignaturePath path, Signer previousOwner) throws Exception{
        version = previous.version + 1;
        if (path == null){
            path = new SignaturePath(previous, previousOwner, SignaturePath.OWNER);
        }
        signature = new DarcSignature(ID(), path, previousOwner);
    }

    /**
     * Returns true if the current darc has correctly been evolved from the previous darc.
     *
     * @param previous
     * @return
     */
    public boolean VerifyEvolution(Darc previous) throws Exception{
        if (signature == null){
            return false;
        }
        return signature.Verify(ID(), previous);
    }

    /**
     * Creates a protobuf representation of the darc.
     *
     * @return the protobuf representation of the darc.
     */
    public DarcProto.Darc ToProto() {
        DarcProto.Darc.Builder b = DarcProto.Darc.newBuilder();
        for (Identity i : owners) {
            b.addOwners(i.ToProto());
        }
        for (Identity i : users) {
            b.addUsers(i.ToProto());
        }
        if (signature != null) {
            b.setSignature(signature.ToProto());
        }
        b.setVersion(version);
        if (data != null) {
            b.setDescription(ByteString.copyFrom(data));
        }
        return b.build();
    }

    /**
     * Adds a user to the list of allowed signers.
     *
     * @param identity
     */
    public void AddUser(Identity identity) {
        users.add(identity);
    }

    /**
     * Adds a user to the list of allowed signers.
     *
     * @param signer
     */
    public void AddUser(Signer signer) throws Exception {
        AddUser(IdentityFactory.New(signer));
    }

    /**
     * Adds a owner to the list of allowed signers.
     *
     * @param identity
     */
    public void AddOwner(Identity identity) {
        owners.add(identity);
    }

    /**
     * Adds a owner to the list of allowed signers.
     *
     * @param signer
     */
    public void AddOwner(Signer signer) throws Exception {
        AddOwner(IdentityFactory.New(signer));
    }

    /**
     * Increments the version of this Darc by 1.
     */
    public void IncVersion(){
        version++;
    }

    /**
     * Returns the current version
     * @return
     */
    public int GetVersion(){
        return version;
    }

    public boolean equals(Darc d){
        return Arrays.equals(this.ID(), d.ID());
    }
}
