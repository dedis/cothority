package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.CothorityCommunicationException;
import ch.epfl.dedis.lib.Crypto;
import ch.epfl.dedis.proto.OCSProto;
import com.google.protobuf.ByteString;

import javax.crypto.Cipher;
import javax.crypto.spec.IvParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.security.SecureRandom;
import java.util.Arrays;

/**
 * dedis/lib
 * Document.java
 * Purpose: Wrapping all fields necessary for a document.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */

public class Document {
    // dataEnc is the encrypted data that can be decrypted using the
    // keyMaterial.
    public byte[] dataEnc;
    // keyMaterial holds the symmetric symmetricKey and eventually an IV for the
    // encryption.
    public byte[] keyMaterial;
    // owner is a pointer to the Darc describing the owner of the document.
    // A Darc allows to change the read-access at a later time in multiple
    // levels, so that either a simple public-symmetricKey list can be implemented
    // or a multi-level publishers/consumers.
    public Darc owner;
    // extraData can be anything that is stored in clear on the skipchain.
    // This part will not be encrypted!
    public byte[] extraData;
    // id of the document is unique - for the moment it is the skipblock where
    // the document is stored - later a counter will be added to differentiate
    // multiple write-transactions per block.
    public byte[] id;

    public static String algo = "AES/CBC/PKCS5Padding";
    public static String algoKey = "AES";
    public static int ivLength = 16;

    public OCSProto.OCSWrite ocswrite;

    /**
     * Creates a new document - copies from an existing.
     *
     * @param doc - an existing document.
     */
    public Document(Document doc) {
        id = doc.id;
        dataEnc = doc.dataEnc;
        extraData = doc.extraData;
        keyMaterial = doc.keyMaterial;
        owner = doc.owner;
    }

    /**
     * Creates a new document from data, creates a new Darc, a symmetric
     * symmetricKey and encrypts the data using CBC-RSA.
     *
     * @param data   - any data that will be stored encrypted on the skipchain.
     *               There is a 10MB-limit on how much data can be stored. If you
     *               need more, this must be a pointer to an off-chain storage.
     * @param keylen - how long the symmetric symmetricKey should be, in bytes. 16 bytes
     *               should be a safe guess for the moment.
     * @throws Exception in the case the encryption doesn't work
     */
    public Document(byte[] data, int keylen) throws Exception {
        keyIv key = new keyIv(keylen);
        this.keyMaterial = key.getKeyMaterial();
        this.dataEnc = encryptData(data, key.getKeyMaterial());

        owner = new Darc();

        extraData = "".getBytes();
    }

    /**
     * Overloaded constructor for Document, but taking a string rather than
     * an array of bytes.
     *
     * @param data   - data for the document, will be encrypted
     * @param keylen - keylength - 16 is a good start.
     */
    public Document(String data, int keylen) throws Exception {
        this(data.getBytes(), keylen);
    }

    /**
     * Create a new document by giving all possible parameters. This call
     * supposes that the data sent here is already encrypted using the
     * keyMaterial and can be decrypted using keyMaterial.
     *
     * @param dataEnc     the already encrypted data
     * @param keyMaterial the symmetric symmetricKey plus eventually an IV. This
     *                    will be encrypted under the shared symmetricKey of the
     *                    cothority
     * @param owner       the owner is allowed to give access to the document
     * @param extraData   data that will _not be encrypted_ but will be
     *                    visible in cleartext on the skipchain
     */
    public Document(byte[] dataEnc, byte[] keyMaterial, Darc owner,
                    byte[] extraData) {
        this.dataEnc = dataEnc;
        this.keyMaterial = keyMaterial;
        this.owner = owner;
        this.extraData = extraData;
    }

    /**
     * Returns a protobuf-formatted block that can be sent to the cothority
     * for storage on the skipchain. The data and the keyMaterial will be
     * encrypted and stored encrypted on the skipchain. The id, extraData and
     * darc however will be stored in clear.
     *
     * @param X - the public symmetricKey of the ocs-shard
     * @return - OCSWrite to be sent to the cothority
     * @throws Exception
     */
    public OCSProto.OCSWrite getWrite(Crypto.Point X) throws CothorityCommunicationException {
        if (ocswrite != null) {
            return ocswrite;
        }
        OCSProto.OCSWrite.Builder write = OCSProto.OCSWrite.newBuilder();
        write.setExtraData(ByteString.copyFrom(extraData));

        try {
            write.setData(ByteString.copyFrom(dataEnc));

            Crypto.KeyPair randkp = new Crypto.KeyPair();
            Crypto.Scalar r = randkp.Scalar;
            Crypto.Point U = randkp.Point;
            write.setU(U.toProto());

            Crypto.Point C = X.scalarMult(r);
            for (int from = 0; from < keyMaterial.length; from += Crypto.pubLen) {
                int to = from + Crypto.pubLen;
                if (to > keyMaterial.length) {
                    to = keyMaterial.length;
                }
                Crypto.Point keyPoint = Crypto.Point.pubStore(Arrays.copyOfRange(keyMaterial, from, to));
                Crypto.Point Cs = C.add(keyPoint);
                write.addCs(Cs.toProto());
            }

            ocswrite = write.build();
            return write.build();

        } catch (Crypto.CryptoException e) {
            throw new CothorityCommunicationException("Encryption problem" + e.getMessage(), e);
        }
    }

    private static class keyIv{
        public byte[] symmetricKey;
        public byte[] iv;
        public IvParameterSpec ivSpec;
        public SecretKeySpec keySpec;

        public keyIv(byte[] keyMaterial) throws Exception{
            int symmetricLength = keyMaterial.length - ivLength;
            if (symmetricLength <= 0){
                throw new Exception("too short symmetricKey material");
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
     * Encrypts the data using the document-specific encryption.
     * @param data the data to encrypt
     * @param keyMaterial random string of length ivLength + keylength.
     *                    The first ivLength bytes are taken as iv, the
     *                    rest is taken as the symmetric symmetricKey.
     * @return a combined
     * @throws Exception
     */
    public static byte[] encryptData(byte[] data, byte[] keyMaterial) throws Exception{
        keyIv key = new keyIv(keyMaterial);

        Cipher cipher = Cipher.getInstance(Document.algo);
        SecretKeySpec secKey = new SecretKeySpec(key.symmetricKey, Document.algoKey);
        cipher.init(Cipher.ENCRYPT_MODE, secKey, key.ivSpec);
        return cipher.doFinal(data);
    }

    /**
     * This method decrypts the data using the same encryption-method
     * as is defined in the Document(data, keylen) constructor.
     *
     * @param dataEnc the encrypted data from the skipchain
     * @param keyMaterial the decrypted keyMaterial
     * @return decrypted data
     * @throws Exception
     */
    public static byte[] decryptData(byte[] dataEnc, byte[] keyMaterial) throws Exception{
        keyIv key = new keyIv(keyMaterial);
        Cipher cipher = Cipher.getInstance(algo);
        cipher.init(Cipher.DECRYPT_MODE, key.keySpec, key.ivSpec);
        return cipher.doFinal(dataEnc);
    }

    /**
     * Convenience method for use with googles-protobuf bytestring.
     *
     * @param dataEnc as google protobuf bytestring
     * @param keyMaterial the decrypted keyMaterial
     * @return decypted data
     * @throws Exception
     */
    public static byte[] decryptData(ByteString dataEnc, byte[] keyMaterial) throws Exception{
        return decryptData(dataEnc.toByteArray(), keyMaterial);
    }
}
