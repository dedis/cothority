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
    public static final int KEY_LENGTH = 16;
    public static final int IV_LENGTH = 12; // standard IV length for GCM
    public static final int GCM_TLEN = 128;

    /**
     * KeyIV represents a secret key and an IV.
     */
    public static class KeyIv {
        final byte[] symmetricKey;
        final byte[] iv;
        final GCMParameterSpec gcmSpec;
        final SecretKeySpec keySpec;

        /**
         * Construct KeyIV from some keyMaterial.
         *
         * @param keyMaterial must be 28 bytes, the first 12 bytes is used as the IV, the second 16 bytes is the actual key.
         * @throws CothorityCryptoException is something goes wrong.
         */
        public KeyIv(byte[] keyMaterial) throws CothorityCryptoException {
            if (keyMaterial.length != KEY_LENGTH + IV_LENGTH)  {
                throw new CothorityCryptoException("keyMaterial must be 28 bytes");
            }
            iv = new byte[IV_LENGTH];
            System.arraycopy(keyMaterial, 0, iv, 0, IV_LENGTH);
            gcmSpec = new GCMParameterSpec(GCM_TLEN, iv);
            symmetricKey = new byte[KEY_LENGTH];
            keySpec = new SecretKeySpec(symmetricKey, ALGO_KEY);
        }

        /**
         * Construct KeyIV with a random key and IV.
         */
        public KeyIv() {
            symmetricKey = new byte[KEY_LENGTH];
            iv = new byte[IV_LENGTH];
            new SecureRandom().nextBytes(symmetricKey);
            new SecureRandom().nextBytes(iv);
            gcmSpec = new GCMParameterSpec(GCM_TLEN, iv);
            keySpec = new SecretKeySpec(symmetricKey, ALGO_KEY);
        }

        /**
         * Getter for the key material, which is a concatenation of IV and the key.
         */
        public byte[] getKeyMaterial() {
            byte[] keyMaterial = new byte[IV_LENGTH + symmetricKey.length];
            System.arraycopy(iv, 0, keyMaterial, 0, IV_LENGTH);
            System.arraycopy(symmetricKey, 0, keyMaterial, IV_LENGTH, symmetricKey.length);
            return keyMaterial;
        }
    }

    /**
     * Encrypts the data using the encryption defined in the header.
     *
     * @param data        the data to encrypt
     * @param keyMaterial random string of length IV_LENGTH + keylength.
     *                    The first IV_LENGTH bytes are taken as iv, the
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
