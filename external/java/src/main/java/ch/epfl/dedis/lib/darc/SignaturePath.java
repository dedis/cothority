package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcOCSProto;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.List;

public class SignaturePath {
    public static final int OWNER = 0;
    public static final int USER = 1;

    private List<Darc> path;
    private Identity signer;
    private int role;

    private final Logger logger = LoggerFactory.getLogger(SignaturePath.class);

    /**
     * The role is the final role this signature should be used.
     * 0 for owner and 1 for user. In the example given in the
     * Signature-class, if the document only knows the id of the version
     * 0 of the reader darc, then the darcPath will be:
     * Reader.0, Reader.1, Publisher.0, Publisher.1
     *
     * @param darcPath
     * @param signer
     * @param role
     */
    public SignaturePath(List<Darc> darcPath, Identity signer, int role) {
        path = darcPath;
        this.signer = signer;
        this.role = role;
    }

    /**
     * Overloaded method for convenience.
     *
     * @param darcPath
     * @param signer
     * @param role
     */
    public SignaturePath(List<Darc> darcPath, Signer signer, int role) throws CothorityCryptoException {
        this(darcPath, IdentityFactory.New(signer), role);
    }

    /**
     * Overloaded method for convenience.
     * @param darc
     * @param signer
     * @param role
     * @throws Exception
     */
    public SignaturePath(Darc darc, Signer signer, int role) throws CothorityCryptoException {
        path = new ArrayList<>();
            path.add(darc);
        this.signer = IdentityFactory.New(signer);
        this.role = role;
    }

    /**
     * For creating online paths that don't inlcude the previous darcs
     * but that need the path to be verified by the verifier itself.
     * @param signer
     * @param role
     * @throws Exception
     */
    public SignaturePath(Signer signer, int role) throws CothorityCryptoException {
        path = new ArrayList<>();
        this.signer = IdentityFactory.New(signer);
        this.role = role;
    }

    public SignaturePath(DarcOCSProto.SignaturePath proto) throws CothorityCryptoException{
        role = proto.getRole();
        signer = IdentityFactory.New(proto.getSigner());
        path = new ArrayList<>();
        for (DarcOCSProto.Darc d :
                proto.getDarcsList()) {
            path.add(new Darc(d));
        }
    }

    /**
     * Returns the path as an array of bytes. For the example given
     * under the Signature-class, the following array would be returned
     * for an owner-signature.
     * byte(0) | Reader.0.getId | Reader.1.getId | Publisher.0.getId | Publisher.1.getId
     *
     * @return
     */
    public byte[] getPathMsg() throws CothorityCryptoException {
        if (path.size() == 0){
            return "online".getBytes();
        }
        byte[] pathMsg = new byte[path.size() * path.get(0).getId().getId().length];
        int pos = 0;
        for (Darc d : path) {
            System.arraycopy(d.getId().getId(), 0, pathMsg, pos * 32, 32);
            pos++;
        }
        return pathMsg;
    }

    public List<DarcId> getPathIDs() {
        List<DarcId> ids = new ArrayList<>();
        for (Darc d :
                path) {
            try {
                ids.add(d.getId());
            } catch (CothorityCryptoException e){
            }
        }
        return ids;
    }

    /**
     * Returns a copy of the darc-list
     * @return
     */
    public List<Darc> getDarcs(){
        List<Darc> darcs = new ArrayList<>();
        for (Darc d :
                path) {
            darcs.add(d);
        }
        return darcs;

    }

    /**
     * Returns the signer of this signature.
     * @return
     */
    public Identity getSigner() {
        return signer;
    }

    /**
     * Creates a protobuf representation of this signature.
     * @return
     */
    public DarcOCSProto.SignaturePath toProto(){
        DarcOCSProto.SignaturePath.Builder b = DarcOCSProto.SignaturePath.newBuilder();
        b.setRole(role);
        b.setSigner(signer.toProto());
        for (Darc d :
                path) {
            b.addDarcs(d.toProto());
        }
        return b.build();
    }

    @Override
    public boolean equals(Object other) {
        if (other == null) return false;
        if (other == this) return true;
        if (!(other instanceof SignaturePath))return false;
        SignaturePath otherPath = (SignaturePath) other;
        boolean paths = true;
        for (int i = 0; i < path.size(); i++){
            if (!path.get(i).equals(otherPath.path.get(i))){
                paths = false;
                break;
            }
        }
        return otherPath.role == role &&
                otherPath.signer.equals(signer) &&
            paths;
    }
}
