package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.darc.SignerX509EC;
import org.apache.commons.io.IOUtils;

import java.io.IOException;
import java.io.InputStream;
import java.security.*;
import java.security.KeyPair;
import java.security.spec.ECGenParameterSpec;
import java.security.spec.InvalidKeySpecException;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.X509EncodedKeySpec;

public class TestSignerX509EC extends SignerX509EC {
    private final PublicKey publicKey;
    private final PrivateKey privateKey;

    /**
     * this constructor create internally random key
     */
    public TestSignerX509EC() {
        this("secp256r1");
    }

    /**
     * Generate test key with specified curve
     *
     * @param keyType type of curve can be one of "secp256r1", "secp384r1", "secp521r1"
     */
    public TestSignerX509EC(String keyType) {
        this(createKeyPair(keyType));
    }

    public TestSignerX509EC(java.security.KeyPair keyPair) {
        this(keyPair.getPublic(), keyPair.getPrivate());
    }

    public TestSignerX509EC(PublicKey publicKey, PrivateKey privateKey) {
        this.publicKey = publicKey;
        this.privateKey = privateKey;
    }

    /**
     * Create a test signger from sample files stored in resources. It accepts keys generated in this way
     *
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
     *
     * @param privateKeyResourceName resource name of a private key in pkcs8 der format
     * @param publicKeyResourceName resource name of a public key x509 der format
     * @throws IOException when key pair can not be read
     */
    public TestSignerX509EC(String privateKeyResourceName, String publicKeyResourceName) throws IOException {
        try {
            publicKey = readPublicKey(getClass().getClassLoader().getResourceAsStream(publicKeyResourceName));
            privateKey = readPrivateKey(getClass().getClassLoader().getResourceAsStream(privateKeyResourceName));
        } catch (InvalidKeySpecException | NoSuchAlgorithmException e) {
            // this is test class so exceptions can be collapsed to a single type
            throw new IOException("Unable to read certificate", e);
        }
    }

    @Override
    public byte[] sign(byte[] msg) throws SignRequestRejectedException {
        final Signature signature;
        try {
            signature = Signature.getInstance("SHA384withECDSA");
            signature.initSign(privateKey);
            signature.update(msg);
            return signature.sign();
        } catch (NoSuchAlgorithmException | SignatureException | InvalidKeyException e) {
            throw new SignRequestRejectedException("unable to sign request", e);
        }
    }

    public PublicKey getPublicKey() {
        return publicKey;
    }

    public static PrivateKey readPrivateKey(InputStream privKeyStream) throws IOException, NoSuchAlgorithmException, InvalidKeySpecException {
        byte privKeyBytes[] = IOUtils.toByteArray(privKeyStream);

        KeyFactory keyFactory = KeyFactory.getInstance("EC");

        PKCS8EncodedKeySpec privSpec = new PKCS8EncodedKeySpec(privKeyBytes);
        return keyFactory.generatePrivate(privSpec);
    }

    public static PublicKey readPublicKey(InputStream pubKeyStream) throws IOException, NoSuchAlgorithmException, InvalidKeySpecException {
        byte pubKeyBytes[] = IOUtils.toByteArray(pubKeyStream);
        KeyFactory keyFactory = KeyFactory.getInstance("EC");

        X509EncodedKeySpec pubSpec = new X509EncodedKeySpec(pubKeyBytes);
        PublicKey pubKey = keyFactory.generatePublic(pubSpec);
        return pubKey;
    }

    private static KeyPair createKeyPair(String algName) {
        try {
            ECGenParameterSpec ecGenParameterSpec = new ECGenParameterSpec(algName);
            KeyPairGenerator keyPairGenerator = KeyPairGenerator.getInstance("EC"); // Elliptic Curve
            keyPairGenerator.initialize(ecGenParameterSpec);
            return keyPairGenerator.generateKeyPair();
        } catch (InvalidAlgorithmParameterException | NoSuchAlgorithmException e) {
            throw new IllegalStateException("Something is not good with your JDK and it is not able to create a key in your format", e);
        }
    }
}

