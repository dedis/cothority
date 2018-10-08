package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import com.google.protobuf.InvalidProtocolBufferException;

import java.util.Arrays;

/**
 * A document is an example how to use Calypso to store a document securely on Byzcoin. The document class always
 * holds the data unencrypted, but encrypts
 * - the data using the keyMaterial
 * - the keyMaterial using the public key of the Long Term Secrets
 * <p>
 * It can return the document if it is given a ReadInstance and the private key to decrypt the keyMaterial.
 */
public class Document {
    private byte[] data;
    private byte[] keyMaterial;
    private byte[] extraData;
    private DarcId publisherId;

    /**
     * Initialises a document. The data already has to be encrypted.
     *
     * @param data        the data that will be encrypted using the keyMaterial
     * @param keyMaterial key-material necessary for the unencryption of the document.
     *                    This will be encrypted before the transmission to the skipchain.
     * @param extraData   any public data that will be stored unencrypted on the skipchain.
     * @param publisherId the publisher darc with the rules to create a WriteInstance and a ReadInstance.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Document(byte[] data, byte[] keyMaterial, byte[] extraData, DarcId publisherId) throws CothorityCryptoException {
        this.data = data;
        this.keyMaterial = keyMaterial;
        this.publisherId = publisherId;
        this.extraData = extraData;
    }

    /**
     * Creates a new document from data, creates a new Darc, a symmetric
     * symmetricKey and encrypts the data using CBC-RSA.
     *
     * @param data        any data that will be stored encrypted on the skipchain.
     *                    There is a 10MB-limit on how much data can be stored. If you
     *                    need more, this must be a pointer to an off-chain storage.
     * @param keylen      how long the symmetric symmetricKey should be, in bytes. 16 bytes
     *                    should be a safe guess for the moment.
     * @param extraData   any public data that will not be encrypted
     * @param publisherId the publisher darc with the rules to create a WriteInstance and a ReadInstance.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Document(byte[] data, int keylen, byte[] extraData, DarcId publisherId) throws CothorityCryptoException {
        this(data, new Encryption.keyIv(keylen).getKeyMaterial(), extraData, publisherId);
    }

    /**
     * Creates a WriteData object with the fields of the document.
     *
     * @param lts the Long Term Secret to use.
     * @return a WriteData with the encrypted data
     * @throws CothorityException if something goes wrong
     */
    public WriteData getWriteData(LTS lts) throws CothorityException {
        return new WriteData(lts, Encryption.encryptData(data, keyMaterial), keyMaterial, extraData, publisherId);
    }

    /**
     * Creates a new WriteInstance with the document stored in it.
     *
     * @param calypso         an instance of the calypsoRPC
     * @param publisherDarcId a darc with a 'spawn:calypsoWrite' rule
     * @param publisherSigner a signer having the right to trigger the 'spawn:calypsoWrite' rule.
     * @return a WriteInstance
     * @throws CothorityException if something goes wrong
     */
    public WriteInstance spawnWrite(CalypsoRPC calypso, DarcId publisherDarcId, Signer publisherSigner) throws CothorityException {
        return new WriteInstance(calypso, publisherDarcId, Arrays.asList(publisherSigner), getWriteData(calypso.getLTS()));
    }

    /**
     * Compares this document with another document. If this document has a
     * writeRequestId, it will be compared against the other document's
     * writeRequestId, else it will be ignored.
     *
     * @param other another Document
     * @return true if both are equal
     */
    @Override
    public boolean equals(Object other) {
        if (other == null) return false;
        if (other == this) return true;
        if (!(other instanceof Document)) return false;
        Document otherDoc = (Document) other;
        boolean wrid = true;
        return Arrays.equals(otherDoc.data, data) &&
                Arrays.equals(otherDoc.extraData, extraData) &&
                Arrays.equals(otherDoc.keyMaterial, keyMaterial) &&
                otherDoc.publisherId.equals(publisherId) &&
                wrid;
    }

    /**
     * @return the key material used to encrypt the data
     */
    public byte[] getKeyMaterial() {
        return keyMaterial;
    }

    /**
     * @return the public data that is stored in cleartext on ByzCoin
     */
    public byte[] getExtraData() {
        return extraData;
    }

    /**
     * @return the darcId protecting access to this document.
     */
    public DarcId getPublisherId() {
        return publisherId;
    }

    /**
     * @return the decrypted data stored in this document.
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public byte[] getData() throws CothorityCryptoException {
        return data;
    }

    /**
     * @return a nicely formatted string representing the document.
     */
    public String toString() {
        String wrid = "null";
        return String.format("data: %s\nextraData: %s\nkeyMaterial: %s\n" +
                        "publisherId: %s\nwriteRequestId: %s",
                Hex.printHexBinary(data),
                Hex.printHexBinary(extraData),
                Hex.printHexBinary(keyMaterial),
                publisherId.toString(),
                wrid);
    }

    /**
     * Fetches a document from calypso, once the read instance has been created. It fetches the ReadInstance,
     * WriteInstance, and the reader darc from ByzCoin, and then re-creates a document with the decrypted keyMaterial
     * and decrypted data.
     *
     * @param calypso an existing calypso
     * @param riId    the instance Id of the read instance
     * @param reader  the private key of the reader (or ephemeral key)
     * @return the document with the decrypted keyMaterial and decrypted data
     * @throws CothorityException if something goes wrong
     */
    public static Document fromCalypso(CalypsoRPC calypso, InstanceId riId, Scalar reader) throws CothorityException {
        ReadInstance ri = ReadInstance.fromByzCoin(calypso, riId);
        WriteInstance wi = WriteInstance.fromCalypso(calypso, ri.getRead().getWriteId());
        byte[] keyMaterial = ri.decryptKeyMaterial(reader);

        return fromWriteInstance(wi, keyMaterial);
    }

    /**
     * Recreate the document once all the material already is fetched from ByzCoin.
     *
     * @param wi WriteInstance for this document
     * @param keyMaterial the decrypted key material
     * @return a new Document with the decrypted data
     * @throws CothorityNotFoundException if the requested instance cannot be found
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public static Document fromWriteInstance(WriteInstance wi, byte[] keyMaterial) throws CothorityNotFoundException, CothorityCryptoException {
        byte[] data = Encryption.decryptData(wi.getWrite().getDataEnc(), keyMaterial);
        return new Document(data, keyMaterial, wi.getWrite().getExtraData(), wi.getDarcId());
    }
}
