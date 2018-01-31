package ch.epfl.dedis;

import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.ValueSource;

import java.security.InvalidAlgorithmParameterException;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.NoSuchAlgorithmException;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.Signature;
import java.security.spec.ECGenParameterSpec;
import java.util.Random;

import static ch.epfl.dedis.lib.darc.TestKeycardSigner.readPrivateKey;
import static ch.epfl.dedis.lib.darc.TestKeycardSigner.readPublicKey;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Just a couple of examples of key creation, loading, data signing and signature verification
 *
 */
public class KeyAndSignOperationsTest {

    @ParameterizedTest
    @ValueSource(strings = { "secp256r1", "secp384r1", "secp521r1" })
    public void exampleKeyGen(String algName) throws NoSuchAlgorithmException, InvalidAlgorithmParameterException {
        KeyPair keyPair = createElipticKeyPair(algName);
        System.out.println(keyPair);
        System.out.println(keyPair.getPrivate());
        System.out.println(keyPair.getPublic());
    }

    private static KeyPair createElipticKeyPair(final String algorithm) throws NoSuchAlgorithmException, InvalidAlgorithmParameterException {
        ECGenParameterSpec ecGenParameterSpec = new ECGenParameterSpec(algorithm);
        KeyPairGenerator keyPairGenerator = KeyPairGenerator.getInstance("EC"); // Elliptic Curve
        keyPairGenerator.initialize(ecGenParameterSpec);
        KeyPair keyPair = keyPairGenerator.generateKeyPair();

        return keyPair;
    }

    /**
     * <h1>private key generation</h1>
     * <p>
     * <pre>
     *     openssl ecparam -name secp256r1 -genkey -noout -outform der -out secp256k1-key.der
     *     openssl ecparam -name secp384r1 -genkey -noout -outform der -out secp384r1-key.der
     *     openssl ecparam -name secp521r1 -genkey -noout -outform der -out secp521r1-key.der
     * </pre>
     * <h2>and conversion to PKCS8</h2>
     * <pre>
     *      openssl pkcs8 -topk8 -nocrypt -outform der -inform der -in secp256k1-key.der -out secp256k1-pkcs8.der
     *      openssl pkcs8 -topk8 -nocrypt -outform der -inform der -in secp384r1-key.der -out secp384r1-pkcs8.der
     *      openssl pkcs8 -topk8 -nocrypt -outform der -inform der -in secp521r1-key.der -out secp521r1-pkcs8.der
     * </pre>
     * <p>
     * <h1>public key generation</h1>
     * <pre>
     *     openssl ec -inform der -in secp256k1-key.der -pubout -outform der -out secp256k1-pub.der
     *     openssl ec -inform der -in secp384r1-key.der -pubout -outform der -out secp384r1-pub.der
     *     openssl ec -inform der -in secp521r1-key.der -pubout -outform der -out secp521r1-pub.der
     * </pre>
     */
    @ParameterizedTest
    @CsvSource({ "secp256k1-pkcs8.der, secp256k1-pub.der",
            "secp384r1-pkcs8.der, secp384r1-pub.der",
            "secp521r1-pkcs8.der, secp521r1-pub.der" })
    public void exampleReadOpenSSLKeyAndSginAndVerify(String priv, String pub) throws Exception {
        byte[] text = new byte[666];
        new Random().nextBytes(text);

        PublicKey publicKey = readPublicKey(getClass().getClassLoader().getResourceAsStream(pub));
        PrivateKey privateKey = readPrivateKey(getClass().getClassLoader().getResourceAsStream(priv));

        assertNotNull(publicKey);
        assertNotNull(privateKey);

        // sign some body
        final Signature signature1 = Signature.getInstance("SHA384withECDSA");
        signature1.initSign(privateKey);
        signature1.update(text);
        final byte[] signature = signature1.sign();

        // verify signature
        final Signature signature2 = Signature.getInstance("SHA384withECDSA");
        signature2.initVerify(publicKey);
        signature2.update(text);
        boolean verificationResults = signature2.verify(signature);

        //
        assertTrue(verificationResults);
    }
}