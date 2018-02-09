package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.List;

public class SignaturePath {
    public static final int OWNER = 0;
    public static final int USER = 1;

    private List<DarcId> path;
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
    public SignaturePath(List<DarcId> darcPath, Identity signer, int role) {
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
    public SignaturePath(List<DarcId> darcPath, Signer signer, int role) throws CothorityCryptoException {
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
        path.add(darc.getId());
        this.signer = IdentityFactory.New(signer);
        this.role = role;
    }

    public SignaturePath(DarcProto.SignaturePath proto) throws CothorityCryptoException{
        role = proto.getRole();
        signer = IdentityFactory.New(proto.getSigner());
        path = new ArrayList<>();
        for (ByteString id :
                proto.getDarcidsList()) {
            path.add(new DarcId(id.toByteArray()));
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
            return new byte[0];
        }
        byte[] pathMsg = new byte[path.size() * path.get(0).getId().length];
        int pos = 0;
        for (DarcId id : path) {
            System.arraycopy(id.getId(), 0, pathMsg, pos * 32, 32);
            pos++;
        }
        return pathMsg;
    }

    public List<DarcId> getPathIDs() {
        List<DarcId> ids = new ArrayList<>();
        for (DarcId id :
                path) {
                ids.add(id);
        }
        return ids;
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
    public DarcProto.SignaturePath toProto(){
        DarcProto.SignaturePath.Builder b = DarcProto.SignaturePath.newBuilder();
        b.setRole(role);
        b.setSigner(signer.toProto());
        for (DarcId id :
                path) {
            b.addDarcids(ByteString.copyFrom(id.getId()));
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
