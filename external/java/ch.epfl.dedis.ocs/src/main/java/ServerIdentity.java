import net.i2p.crypto.eddsa.spec.EdDSANamedCurveTable;
import net.i2p.crypto.eddsa.spec.EdDSAPublicKeySpec;
import net.i2p.crypto.eddsa.EdDSAPublicKey;

import java.security.PublicKey;
import java.util.Base64;

import com.moandjiezana.toml.*;

public class ServerIdentity {
    public String Address;
    public String Description;
    public PublicKey Public;

    public ServerIdentity(String definition) {
        this(new Toml().read(definition).getTables("servers").get(0));
    }

    public ServerIdentity(Toml t){
        this.Address = t.getString("Address");
        this.Description = t.getString("Description");
        String pub = t.getString("Public");
        byte[] pubBytes = Base64.getDecoder().decode(pub);
        EdDSAPublicKeySpec spec = new EdDSAPublicKeySpec(pubBytes,
                EdDSANamedCurveTable.getByName("ed25519"));
        this.Public = new EdDSAPublicKey(spec);
    }
}
