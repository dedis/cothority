import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.spec.ECGenParameterSpec;

public class Account {
    public enum AccessRight {
        ADMIN, WRITER, READER
    }

    private byte[] ID;
    private PublicKey Pub;
    private PrivateKey Priv;
    private AccessRight Access;

    public Account(AccessRight a) throws Exception{
        this.Access = a;
        KeyPairGenerator kpg;
        kpg = KeyPairGenerator.getInstance("EC","SunEC");
        ECGenParameterSpec ecsp;
        ecsp = new ECGenParameterSpec("secp192r1");
        kpg.initialize(ecsp);

        KeyPair kp = kpg.genKeyPair();
        this.Priv = kp.getPrivate();
        this.Pub = kp.getPublic();
    }
}

