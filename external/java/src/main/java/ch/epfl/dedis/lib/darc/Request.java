package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.proto.DarcProto;
import com.google.protobuf.ByteString;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.List;

/**
 * The darc request, which represents a request or action that the client signs off for the service to verify.
 */
public class Request {
    private DarcId baseId;
    private String action;
    private byte[] msg;
    private List<Identity> identities;
    private List<byte[]> signatures;

    /**
     * Constructor for the darc request.
     * @param baseId The base ID of the darc, the service will verify using the latest darc for this base ID.
     * @param action The action of the request, it is typically one of the rules in the darc.
     * @param msg The message that the service will check, typically a hash of some payload that is not contained in
     *            the request itself.
     * @param identities The identities of the signers.
     * @param signatures The signature of the signers. It is not essential to set these in the constructor, they can
     *                   always be set using the setter.
     */
    public Request(DarcId baseId, String action, byte[] msg, List<Identity> identities, List<byte[]> signatures) {
        this.baseId = baseId;
        this.action = action;
        this.msg = msg;
        this.identities = identities;
        this.signatures = signatures;
    }

    /**
     * The setter for signatures.
     * @param signatures These signatures must be on the digest of the request, i.e. output of the hash method.
     */
    public void setSignatures(List<byte[]> signatures) {
        this.signatures = signatures;
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public DarcProto.Request toProto() {
        DarcProto.Request.Builder b = DarcProto.Request.newBuilder();
        b.setBaseid(ByteString.copyFrom(this.baseId.getId()));
        b.setAction(this.action);
        b.setMsg(ByteString.copyFrom(msg));
        for (Identity id : this.identities) {
            b.addIdentities(id.toProto());
        }
        for (byte[] s : this.signatures) {
            b.addSignatures(ByteString.copyFrom(s));
        }
        return b.build();
    }

    /**
     * Computes the sha256 digest of the request, the message that it hashes does not include the signature part of the
     * request.
     * @return The digest.
     */
    public byte[] hash() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            if (this.baseId != null) {
                digest.update(this.baseId.getId());
            }
            digest.update(this.action.getBytes());
            digest.update(this.msg);
            this.identities.forEach((id) -> {
                digest.update(id.toString().getBytes());
            });
            return digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }
}
