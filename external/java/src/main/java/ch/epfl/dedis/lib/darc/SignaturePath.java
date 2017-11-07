package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

public class SignaturePath {
    public static int OWNER = 0;
    public static int USER = 1;

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
    public SignaturePath(List<Darc> darcPath, Signer signer, int role) throws Exception {
        this(darcPath, IdentityFactory.New(signer), role);
    }

    /**
     * Overloaded method for convenience.
     * @param darc
     * @param signer
     * @param role
     * @throws Exception
     */
    public SignaturePath(Darc darc, Signer signer, int role) throws Exception {
        path = new ArrayList<>();
        path.add(darc);
        this.signer = IdentityFactory.New(signer);
        this.role = role;
    }

    public SignaturePath(DarcProto.SignaturePath proto) throws Exception{
        role = proto.getRole();
        signer = IdentityFactory.New(proto.getSigner());
        path = new ArrayList<>();
        for (DarcProto.Darc d :
                proto.getDarcsList()) {
            path.add(new Darc(d));
        }
    }

    /**
     * Returns the path as an array of bytes. For the example given
     * under the Signature-class, the following array would be returned
     * for an owner-signature.
     * byte(0) | Reader.0.ID | Reader.1.ID | Publisher.0.ID | Publisher.1.ID
     *
     * @return
     */
    public byte[] GetPathMsg() throws IOException {
        if (path.size() == 0){
            return new byte[0];
        }
        byte[] pathMsg = new byte[path.size() * path.get(0).ID().length];
        int pos = 0;
        for (Darc d : path) {
            System.arraycopy(d.ID(), 0, pathMsg, pos * 32, 32);
            pos++;
        }
        return pathMsg;
    }

    public List<byte[]> GetPathIDs() {
        List<byte[]> ids = new ArrayList<>();
        for (Darc d :
                path) {
            ids.add(d.ID());
        }
        return ids;
    }

    /**
     * Returns a copy of the darc-list
     * @return
     */
    public List<Darc> GetDarcs(){
        List<Darc> darcs = new ArrayList<>();
        for (Darc d :
                path) {
            darcs.add(d);
        }
        return darcs;

    }

    public Identity GetSigner() {
        return signer;
    }

    public DarcProto.SignaturePath ToProto(){
        DarcProto.SignaturePath.Builder b = DarcProto.SignaturePath.newBuilder();
        b.setRole(role);
        b.setSigner(signer.ToProto());
        for (Darc d :
                path) {
            b.addDarcs(d.ToProto());
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
