package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.CothorityCommunicationException;
import ch.epfl.dedis.lib.Crypto;
import ch.epfl.dedis.proto.OCSProto;
import com.google.protobuf.ByteString;

import javax.crypto.BadPaddingException;
import javax.crypto.Cipher;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import javax.crypto.spec.SecretKeySpec;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;
import java.util.Random;

/**
 * dedis/lib
 * Document.java
 * Purpose: Wrapping all fields necessary for a document.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */

public class Document {
    // data is the data that will be encrypted by the symmetric key.
    public byte[] data;
    // symmetricKey is initialised for each new document with a random
    // value. You can overwrite this value before calling `getWrite`.
    public byte[] symmetricKey;
    // readers is a pointer to the Darc holding reader-access. The Darc
    // can hold an empty reader-list when storing the document, but must
    // include at least one administrator for the Darc. The Darc-administrator
    // is allowed to make changes to the readers.
    public Darc readers;
    // extraData can be anything that is stored in clear on the skipchain.
    // This part will not be encrypted!
    public byte[] extraData;
    // id of the document is unique - for the moment it is the skipblock where
    // the document is stored - later a counter will be added to differentiate
    // multiple write-transactions per block.
    public byte[] id;

    public OCSProto.OCSWrite ocswrite;

    /**
     * Creates a new document - copies from an existing.
     *
     * @param doc - an existing document.
     */
    public Document(Document doc) {
        id = doc.id;
        data = doc.data;
        extraData = doc.extraData;
        symmetricKey = doc.symmetricKey;
        readers = doc.readers;
    }

    /**
     * Creates a new document from data and initializes the symmetric key.
     *
     * @param data   - any data that will be stored encrypted on the skipchain.
     *               There is a 10MB-limit on how much data can be stored. If you
     *               need more, this must be a pointer to an off-chain storage.
     * @param keylen - how long the symmetric key should be, in bytes. 16 bytes
     *               should be a safe guess for the moment.
     */
    public Document(byte[] data, int keylen) {
        this.data = data;
        symmetricKey = new byte[keylen];
        new Random().nextBytes(symmetricKey);
        extraData = "".getBytes();
    }

    /**
     * Overloaded constructor for Document, but taking a string rather than
     * an array of bytes.
     *
     * @param data   - data for the document
     * @param keylen - keylength - 16 is a good start.
     */
    public Document(String data, int keylen) {
        this(data.getBytes(), keylen);
    }

    /**
     * Returns a protobuf-formatted block that can be sent to the cothority
     * for storage on the skipchain. The data and the symmetricKey will be
     * encrypted and stored encrypted on the skipchain. The id, extraData and
     * darc however will be stored in clear.
     *
     * @param X - the public key of the ocs-shard
     * @return - OCSWrite to be sent to the cothority
     * @throws Exception
     */
    public OCSProto.OCSWrite getWrite(Crypto.Point X) throws CothorityCommunicationException {
        if (ocswrite != null) {
            return ocswrite;
        }
        OCSProto.OCSWrite.Builder write = OCSProto.OCSWrite.newBuilder();

        try {
            Cipher cipher = Cipher.getInstance(Crypto.algo);
            cipher.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(symmetricKey, Crypto.algoKey));

            byte[] dataEnc = cipher.doFinal(data);
            write.setData(ByteString.copyFrom(dataEnc));

            Crypto.KeyPair randkp = new Crypto.KeyPair();
            Crypto.Scalar r = randkp.Scalar;
            Crypto.Point U = randkp.Point;
            write.setU(U.toProto());

            Crypto.Point C = X.scalarMult(r);
            for (int from = 0; from < symmetricKey.length; from += Crypto.pubLen) {
                int to = from + Crypto.pubLen;
                if (to > symmetricKey.length) {
                    to = symmetricKey.length;
                }
                Crypto.Point keyPoint = Crypto.Point.pubStore(Arrays.copyOfRange(symmetricKey, from, to));
                Crypto.Point Cs = C.add(keyPoint);
                write.addCs(Cs.toProto());
            }

            ocswrite = write.build();
            return write.build();

        } catch (NoSuchAlgorithmException | NoSuchPaddingException | InvalidKeyException
                | BadPaddingException | IllegalBlockSizeException | Crypto.CryptoException e) {
            throw new CothorityCommunicationException("Encryption problem" + e.getMessage(), e);
        }
    }
}
