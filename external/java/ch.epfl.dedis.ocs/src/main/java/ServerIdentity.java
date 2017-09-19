import com.google.protobuf.ByteString;

import java.security.PublicKey;
import java.util.Base64;

import com.moandjiezana.toml.*;
import proto.ServerIdentityProto;
import proto.StatusProto;

import javax.xml.bind.DatatypeConverter;


public class ServerIdentity {
    public String Address;
    public String Description;
    public Crypto.Point Public;

    public ServerIdentity(String definition) {
        this(new Toml().read(definition).getTables("servers").get(0));
    }

    public ServerIdentity(Toml t) {
        this.Address = t.getString("Address");
        this.Description = t.getString("Description");
        String pub = t.getString("Point");
        byte[] pubBytes = Base64.getDecoder().decode(pub);
        this.Public = new Crypto.Point(pubBytes);
    }

    public String AddressWebSocket() {
        String[] ipPort = Address.replaceFirst("^tcp://", "").split(":");
        int Port = Integer.valueOf(ipPort[1]);
        return String.format("%s:%d", ipPort[0], Port + 1);
    }

    public StatusProto.Response GetStatus() throws Exception {
        StatusProto.Request request =
                StatusProto.Request.newBuilder().build();
        SyncSendMessage msg = new SyncSendMessage(this, "Status/Request", request.toByteArray());
        if (msg.ok) {
            return StatusProto.Response.parseFrom(msg.response);
        } else {
            System.out.println(msg.error);
        }

        return null;
    }

    public ServerIdentityProto.ServerIdentity getProto(){
        ServerIdentityProto.ServerIdentity.Builder si =
                ServerIdentityProto.ServerIdentity.newBuilder();
        si.setPublic(Public.toProto());
        String pubStr = "https://dedis.epfl.ch/id/" + Public.toString().toLowerCase();
        byte[] id = UUIDType5.toBytes(UUIDType5.nameUUIDFromNamespaceAndString(UUIDType5.NAMESPACE_URL, pubStr));
        si.setId(ByteString.copyFrom(id));
        si.setAddress(Address);
        si.setDescription(Description);
        return si.build();
    }
}
