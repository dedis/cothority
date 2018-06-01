package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

import javax.xml.bind.DatatypeConverter;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.List;

public class Request {
    private DarcId baseId;
    private String action;
    private byte[] msg;
    private List<Identity> identities;
    private List<byte[]> signatures;

    public Request(DarcId baseId, String action, byte[] msg, List<Identity> identities, List<byte[]> signatures) {
        this.baseId = baseId;
        this.action = action;
        this.msg = msg;
        this.identities = identities;
        this.signatures = signatures;
    }

    public void setSignatures(List<byte[]> signatures) {
        this.signatures = signatures;
    }

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

    /*
    public void print() {
        System.out.print("baseId: ");
        if (this.baseId != null) {
            System.out.println(DatatypeConverter.printHexBinary(this.baseId.getId()));
        }
        System.out.println("action: " + this.action);
        System.out.print("msg: ");
        if (this.msg != null) {
            System.out.println(DatatypeConverter.printHexBinary(this.msg));
        }
        this.identities.forEach((id) -> {
            System.out.println(id.toString());
        });
    }
    */
}
