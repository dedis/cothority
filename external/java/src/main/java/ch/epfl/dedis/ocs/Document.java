package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.crypto.Encryption;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;

import javax.xml.bind.DatatypeConverter;
import java.util.Arrays;

import static ch.epfl.dedis.lib.crypto.Encryption.encryptData;

public class Document {
    private byte[] dataEncrypted;
    private byte[] keyMaterial;
    private byte[] dataPublic;
    private WriteRequestId writeRequestId;
    private Darc readers;

    /**
     * Initialises a document. The data already has to be encrypted. The
     * writeRequestId can be null if there hasn't been done a write request
     * yet.
     *
     * @param dataEncrypted  the encrypted data of the document
     * @param keyMaterial    key-material necessary for the unencryption of the document.
     *                       This will be encrypted before the transmission to the skipchain.
     * @param dataPublic     any public data that will be stored unencrypted on the skipchain.
     * @param readers        the readers allowed to create a read-request on this document.
     * @param writeRequestId the writeRequest, if this represents a document already on
     *                       the skipchain.
     */
    public Document(byte[] dataEncrypted, byte[] keyMaterial, byte[] dataPublic, Darc readers,
                    WriteRequestId writeRequestId) {
        this.dataEncrypted = dataEncrypted;
        this.keyMaterial = keyMaterial;
        this.dataPublic = dataPublic;
        this.writeRequestId = writeRequestId;
        this.readers = readers;
    }

    /**
     * Initialises a document. The data already has to be encrypted.
     *
     * @param dataEncrypted the encrypted data of the document
     * @param keyMaterial   key-material necessary for the unencryption of the document.
     *                      This will be encrypted before the transmission to the skipchain.
     * @param dataPublic    any public data that will be stored unencrypted on the skipchain.
     * @param readers       the readers allowed to create a read-request on this document.
     */
    public Document(byte[] dataEncrypted, byte[] keyMaterial, byte[] dataPublic, Darc readers) {
        this(dataEncrypted, keyMaterial, dataPublic, readers, null);
    }

    /**
     * Creates a new document from data, creates a new Darc, a symmetric
     * symmetricKey and encrypts the data using CBC-RSA.
     *
     * @param data       any data that will be stored encrypted on the skipchain.
     *                   There is a 10MB-limit on how much data can be stored. If you
     *                   need more, this must be a pointer to an off-chain storage.
     * @param keylen     how long the symmetric symmetricKey should be, in bytes. 16 bytes
     *                   should be a safe guess for the moment.
     * @param dataPublic any public data that will not be encrypted
     * @throws CothorityCryptoException in the case the encryption doesn't work
     */
    public Document(byte[] data, int keylen, Darc readers, byte[] dataPublic) throws CothorityCryptoException {
        Encryption.keyIv key = new Encryption.keyIv(keylen);
        this.keyMaterial = key.getKeyMaterial();
        this.dataEncrypted = encryptData(data, key.getKeyMaterial());
        this.readers = readers;
        this.dataPublic = dataPublic;
    }

    /**
     * Convenience method without the public data.
     *
     * @param data
     * @param keylen
     * @param readers
     * @throws CothorityCryptoException
     */
    public Document(byte[] data, int keylen, Darc readers) throws CothorityCryptoException {
        this(data, keylen, readers, new byte[]{});
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
        if (writeRequestId != null) {
            wrid = writeRequestId.equals(otherDoc.writeRequestId);
        }
        return Arrays.equals(otherDoc.dataEncrypted, dataEncrypted) &&
                Arrays.equals(otherDoc.dataPublic, dataPublic) &&
                Arrays.equals(otherDoc.keyMaterial, keyMaterial) &&
                otherDoc.readers.equals(readers) &&
                wrid;
    }

    public WriteRequest getWriteRequest() {
        return new WriteRequest(dataEncrypted, keyMaterial, readers, dataPublic);
    }

    public byte[] getDataEncrypted() {
        return dataEncrypted;
    }

    public byte[] getKeyMaterial() {
        return keyMaterial;
    }

    public byte[] getDataPublic() {
        return dataPublic;
    }

    public WriteRequestId getWriteRequestId() {
        return writeRequestId;
    }

    public Darc getReaders() {
        return readers;
    }

    public String toString() {
        String wrid = "null";
        if (writeRequestId != null) {
            wrid = writeRequestId.toString();
        }
        return String.format("dataEncrypted: %s\ndataPublic: %s\nkeyMaterial: %s\n" +
                        "readers: %s\nwriteRequestId: %s",
                DatatypeConverter.printHexBinary(dataEncrypted),
                DatatypeConverter.printHexBinary(dataPublic),
                DatatypeConverter.printHexBinary(keyMaterial),
                readers.toString(),
                wrid);
    }
}
