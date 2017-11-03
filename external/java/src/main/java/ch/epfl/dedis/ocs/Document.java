package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.crypto.Ed25519;
import ch.epfl.dedis.lib.crypto.Encryption;
import ch.epfl.dedis.lib.crypto.KeyPair;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.crypto.Scalar;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.OCSProto;
import com.google.protobuf.ByteString;

import java.util.Arrays;

import static ch.epfl.dedis.lib.crypto.Encryption.encryptData;

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
    public Document(byte[] data, int keylen, Darc owner) throws Exception {
        Encryption.keyIv key = new Encryption.keyIv(keylen);
        this.keyMaterial = key.getKeyMaterial();
        this.dataEnc = encryptData(data, key.getKeyMaterial());

        this.owner = owner;

        extraData = "".getBytes();
    }

    /**
     * Overloaded constructor for Document, but taking a string rather than
     * an array of bytes.
     *
     * @param data   - data for the document, will be encrypted
     * @param keylen - keylength - 16 is a good start.
     */
    public Document(String data, int keylen, Darc owner) throws Exception {
        this(data.getBytes(), keylen, owner);
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
    public OCSProto.Write getWrite(Point X) throws CothorityCommunicationException {
        OCSProto.Write.Builder write = OCSProto.Write.newBuilder();
        write.setExtradata(ByteString.copyFrom(extraData));
        write.setReader(owner.ToProto());

        try {
            write.setData(ByteString.copyFrom(dataEnc));

            KeyPair randkp = new KeyPair();
            Scalar r = randkp.Scalar;
            Point U = randkp.Point;
            write.setU(U.toProto());

            Point C = X.scalarMult(r);
            for (int from = 0; from < keyMaterial.length; from += Ed25519.pubLen) {
                int to = from + Ed25519.pubLen;
                if (to > keyMaterial.length) {
                    to = keyMaterial.length;
                }
                Point keyPoint = Point.pubStore(Arrays.copyOfRange(keyMaterial, from, to));
                Point Cs = C.add(keyPoint);
                write.addCs(Cs.toProto());
            }

            return write.build();

        } catch (CothorityCryptoException e) {
            throw new CothorityCommunicationException("Encryption problem" + e.getMessage(), e);
        }
    }
}
