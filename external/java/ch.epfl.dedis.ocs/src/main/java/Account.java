import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.spec.ECGenParameterSpec;

public class Account {
    public enum AccessRight {
        ADMIN, WRITER, READER
    }

    public byte[] ID;
    public PublicKey Pub;
    public PrivateKey Priv;
    public AccessRight Access;

    public Account(AccessRight a) throws Exception{
        Access = a;
        ID = Crypto.uuid4();

        Crypto.EdKeyPair kp = new Crypto.EdKeyPair();
        Priv = kp.Public;
        Pub = kp.Private;
    }
}

