import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.spec.ECGenParameterSpec;

// The following code is from http://www.academicpub.org/PaperInfo.aspx?PaperID=14496 .
public class ECCKeyGeneration {
    public static void main(String[] args) throws Exception {
        KeyPairGenerator kpg;
        kpg = KeyPairGenerator.getInstance("EC","SunEC");
        ECGenParameterSpec ecsp;
        ecsp = new ECGenParameterSpec("secp192r1");
        kpg.initialize(ecsp);

        KeyPair kp = kpg.genKeyPair();
        PrivateKey privKey = kp.getPrivate();
        PublicKey pubKey = kp.getPublic();

        System.out.println(privKey.toString());
        System.out.println(pubKey.toString());
    }
}

