package ch.epfl.dedis.lib.crypto;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import com.google.protobuf.ByteString;

import javax.crypto.BadPaddingException;
import javax.crypto.Cipher;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import javax.crypto.spec.IvParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;

public class Encryption {
    public static String algo = "AES/CBC/PKCS5Padding";
    public static String algoKey = "AES";
    public static int ivLength = 16;

    public static class keyIv{
        public byte[] symmetricKey;
        public byte[] iv;
        public IvParameterSpec ivSpec;
        public SecretKeySpec keySpec;

        public keyIv(byte[] keyMaterial) throws CothorityCryptoException {
            int symmetricLength = keyMaterial.length - ivLength;
            if (symmetricLength <= 0){
                throw new CothorityCryptoException("too short symmetricKey material");
            }
            iv = new byte[ivLength];
            System.arraycopy(keyMaterial, 0, iv, 0, ivLength);
            ivSpec = new IvParameterSpec(iv);
            symmetricKey = new byte[keyMaterial.length - ivLength];
            keySpec = new SecretKeySpec(symmetricKey, algoKey);
        }

        public keyIv(int keylength){
            symmetricKey = new byte[keylength];
            iv = new byte[ivLength];
            new SecureRandom().nextBytes(symmetricKey);
            new SecureRandom().nextBytes(iv);
            ivSpec = new IvParameterSpec(iv);
            keySpec = new SecretKeySpec(symmetricKey, algoKey);
        }

        public byte[] getKeyMaterial(){
            byte[] keyMaterial = new byte[ivLength + symmetricKey.length];
            System.arraycopy(iv, 0, keyMaterial, 0, ivLength);
            System.arraycopy(symmetricKey, 0, keyMaterial, ivLength, symmetricKey.length);
            return keyMaterial;
        }
    }

    /**
     * Encrypts the data using the encryption defined in the header.
     * @param data the data to encrypt
     * @param keyMaterial random string of length ivLength + keylength.
     *                    The first ivLength bytes are taken as iv, the
     *                    rest is taken as the symmetric symmetricKey.
     * @return a combined
     * @throws Exception
     */
    public static byte[] encryptData(byte[] data, byte[] keyMaterial) throws CothorityCryptoException {
        keyIv key = new keyIv(keyMaterial);

        try {
            Cipher cipher = Cipher.getInstance(Encryption.algo);
            SecretKeySpec secKey = new SecretKeySpec(key.symmetricKey, Encryption.algoKey);
            cipher.init(Cipher.ENCRYPT_MODE, secKey, key.ivSpec);
            return cipher.doFinal(data);
        } catch (NoSuchAlgorithmException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (NoSuchPaddingException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (InvalidKeyException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (InvalidAlgorithmParameterException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (IllegalBlockSizeException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (BadPaddingException e){
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    /**
     * This method decrypts the data using the same encryption-method
     * as is defined in the header of this class.
     *
     * @param dataEnc the encrypted data from the skipchain
     * @param keyMaterial the decrypted keyMaterial
     * @return decrypted data
     * @throws Exception
     */
    public static byte[] decryptData(byte[] dataEnc, byte[] keyMaterial) throws CothorityCryptoException {
        keyIv key = new keyIv(keyMaterial);
        try {
            Cipher cipher = Cipher.getInstance(algo);
            cipher.init(Cipher.DECRYPT_MODE, key.keySpec, key.ivSpec);
            return cipher.doFinal(dataEnc);
        } catch (NoSuchAlgorithmException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (NoSuchPaddingException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (InvalidKeyException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (InvalidAlgorithmParameterException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (IllegalBlockSizeException e){
            throw new CothorityCryptoException(e.getMessage());
        } catch (BadPaddingException e){
            throw new CothorityCryptoException(e.getMessage());
        }
    }

    /**
     * Convenience method for use with googles-protobuf bytestring.
     *
     * @param dataEnc as google protobuf bytestring
     * @param keyMaterial the decrypted keyMaterial
     * @return decypted data
     * @throws Exception
     */
    public static byte[] decryptData(ByteString dataEnc, byte[] keyMaterial) throws CothorityCryptoException {
        return decryptData(dataEnc.toByteArray(), keyMaterial);
    }

}
