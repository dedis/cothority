package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;

import javax.crypto.*;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;

public class Encryption {
    public static final String ALGO = "AES/GCM/NoPadding";
    public static final String ALGO_KEY = "AES";
    public static final int KEY_LEN = 16;
    public static final int IV_LEN = 12; // standard IV length for GCM
    public static final int GCM_TLEN = 128;
    public static final int KEYMATERIAL_LEN = IV_LEN + KEY_LEN;

    /**
     * KeyIV represents a secret key and an IV.
     */
    public static class KeyIv {
        final byte[] symmetricKey;
        final byte[] iv;
        final GCMParameterSpec gcmSpec;
        final SecretKeySpec keySpec;

        /**
         * Construct KeyIV from some keyMaterial, which is the concatenation of IV and Key. The IV must not repeat.
         * If there is no need to use a specific IV and Key, please use the default constructor, and then use
         * getKeyMaterial to get the keyMaterial.
         *
         * @param keyMaterial must be 28 bytes, the first 12 bytes is used as the IV, the second 16 bytes is the actual
         *                    key.
         * @throws CothorityCryptoException is something goes wrong.
         */
        public KeyIv(byte[] keyMaterial) throws CothorityCryptoException {
            if (keyMaterial.length != KEYMATERIAL_LEN)  {
                throw new CothorityCryptoException("keyMaterial must be 28 bytes");
            }
            iv = new byte[IV_LEN];
            System.arraycopy(keyMaterial, 0, iv, 0, IV_LEN);
            gcmSpec = new GCMParameterSpec(GCM_TLEN, iv);
            symmetricKey = new byte[KEY_LEN];
            keySpec = new SecretKeySpec(symmetricKey, ALGO_KEY);
        }

        /**
         * Default construct KeyIV with a random key and IV. Use getKeyMaterial to get the keyMaterial back.
         */
        public KeyIv() {
            symmetricKey = new byte[KEY_LEN];
            iv = new byte[IV_LEN];
            new SecureRandom().nextBytes(symmetricKey);
            new SecureRandom().nextBytes(iv);
            gcmSpec = new GCMParameterSpec(GCM_TLEN, iv);
            keySpec = new SecretKeySpec(symmetricKey, ALGO_KEY);
        }

        /**
         * Getter for the key material, which is a concatenation of IV and the key.
         */
        public byte[] getKeyMaterial() {
            byte[] keyMaterial = new byte[KEYMATERIAL_LEN];
            System.arraycopy(iv, 0, keyMaterial, 0, IV_LEN);
            System.arraycopy(symmetricKey, 0, keyMaterial, IV_LEN, KEY_LEN);
            return keyMaterial;
        }
    }

    /**
     * Encrypts the data using the encryption defined in the header.
     *
     * @param data        the data to encrypt
     * @param keyMaterial random string of length IV_LEN + keylength.
     *                    The first IV_LEN bytes are taken as iv, the
     *                    rest is taken as the symmetric symmetricKey,
     *                    which must be at least 16 bytes long.
     * @return a combined
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public static byte[] encryptData(byte[] data, byte[] keyMaterial) throws CothorityCryptoException {
        KeyIv key = new KeyIv(keyMaterial);
        try {
            Cipher cipher = Cipher.getInstance(Encryption.ALGO);
            cipher.init(Cipher.ENCRYPT_MODE, key.keySpec, key.gcmSpec);
            return cipher.doFinal(data);
        } catch (NoSuchAlgorithmException | NoSuchPaddingException | InvalidAlgorithmParameterException |
                InvalidKeyException | BadPaddingException | IllegalBlockSizeException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    /**
     * This method decrypts the data using the same encryption-method
     * as is defined in the header of this class.
     *
     * @param dataEnc     the encrypted data from the skipchain
     * @param keyMaterial the decrypted keyMaterial
     * @return decrypted data
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public static byte[] decryptData(byte[] dataEnc, byte[] keyMaterial) throws CothorityCryptoException {
        KeyIv key = new KeyIv(keyMaterial);
        try {
            Cipher cipher = Cipher.getInstance(ALGO);
            cipher.init(Cipher.DECRYPT_MODE, key.keySpec, key.gcmSpec);
            return cipher.doFinal(dataEnc);
        } catch (NoSuchAlgorithmException | NoSuchPaddingException | InvalidKeyException | IllegalBlockSizeException |
                InvalidAlgorithmParameterException | BadPaddingException e) {
            throw new CothorityCryptoException(e.getMessage());
        }
    }
}
