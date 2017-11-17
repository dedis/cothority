package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.crypto.*;
import ch.epfl.dedis.lib.darc.Darc;
import ch.epfl.dedis.lib.darc.DarcSignature;
import ch.epfl.dedis.lib.darc.SignaturePath;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.OCSProto;
import com.google.protobuf.ByteString;

import java.util.Arrays;

import static ch.epfl.dedis.lib.crypto.Encryption.encryptData;

/**
 * dedis/lib
 * WriteRequest.java
 * Purpose: Wrapping all fields necessary for a document.
 */

public class WriteRequest {
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
    public WriteRequestId id;

    /**
     * Creates a new document - copies from an existing.
     *
     * @param wr - an existing document.
     */
    public WriteRequest(WriteRequest wr) {
        id = wr.id;
        dataEnc = wr.dataEnc;
        extraData = wr.extraData;
        keyMaterial = wr.keyMaterial;
        owner = wr.owner;
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
     * @throws CothorityCryptoException in the case the encryption doesn't work
     */
    public WriteRequest(byte[] data, int keylen, Darc owner) throws CothorityCryptoException {
        Encryption.keyIv key = new Encryption.keyIv(keylen);
        this.keyMaterial = key.getKeyMaterial();
        this.dataEnc = encryptData(data, key.getKeyMaterial());

        this.owner = owner;

        extraData = "".getBytes();
    }

    /**
     * Overloaded constructor for WriteRequest, but taking a string rather than
     * an array of bytes.
     *
     * @param data   - data for the document, will be encrypted
     * @param keylen - keylength - 16 is a good start.
     */
    public WriteRequest(String data, int keylen, Darc owner) throws CothorityCryptoException {
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
    public WriteRequest(byte[] dataEnc, byte[] keyMaterial, Darc owner,
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
    public OCSProto.Write toProto(Point X) throws CothorityCommunicationException {
        OCSProto.Write.Builder write = OCSProto.Write.newBuilder();
        write.setExtradata(ByteString.copyFrom(extraData));
        write.setReader(owner.toProto());

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

    /**
     * Creates a correct signature-path starting from the admin darc and
     * signs that path to get a correct write-request.
     *
     * @param ocs       OnchainSecretsRPC class to request the path
     * @param publisher allowed to sign for write requests
     * @return a valid signature for a write request
     * @throws CothorityCryptoException
     * @throws CothorityCommunicationException
     */
    public DarcSignature getSignature(OnchainSecretsRPC ocs, Signer publisher) throws CothorityCryptoException, CothorityCommunicationException {
        SignaturePath path = ocs.getDarcPath(ocs.getAdminDarc().getBaseId(),
                publisher.getIdentity(), SignaturePath.USER);
        return new DarcSignature(owner.getId().getId(),
                path, publisher);
    }
}
