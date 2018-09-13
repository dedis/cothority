package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.lib.crypto.*;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.byzcoin.darc.DarcId;
import ch.epfl.dedis.proto.Calypso;
import com.google.protobuf.ByteString;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

import static ch.epfl.dedis.lib.crypto.Encryption.encryptData;

/**
 * A WriteRequest is the data that is sent to the calypsoWrite contract store a write request with the encrypted document.
 */
public class WriteRequest {
    // dataEnc is the encrypted data that can be decrypted using the keyMaterial.
    private byte[] dataEnc;
    // keyMaterial holds the symmetric symmetricKey and eventually an IV for the encryption.
    private byte[] keyMaterial;
    // darcId is the ID of the darc where this write request will be stored.
    private DarcId darcId;
    // extraData can be anything that is stored in clear on the skipchain. This part will not be encrypted!
    private byte[] extraData;

    /**
     * Copy constructor for WriteRequest.
     */
    public WriteRequest(WriteRequest wr) {
        dataEnc = wr.dataEnc;
        keyMaterial = wr.keyMaterial;
        darcId = wr.darcId;
        extraData = wr.extraData;
    }

    /**
     * Creates a new document from data, creates a new Darc, a symmetric symmetricKey and encrypts the data using
     * CBC-RSA.
     *
     * @param data     Plain text data that will be stored encrypted on OmniLedger. There is a 10MB-limit on how much
     *                 data can be stored. If you need more, this must be a pointer to an off-chain storage.
     * @param keylen   The length of the symmetric key in bytes. We recommend using 32 bytes.
     * @param writerID The darc ID where this write request will be stored.
     * @throws CothorityCryptoException in the case the encryption doesn't work
     */
    public WriteRequest(byte[] data, int keylen, DarcId writerID) throws CothorityCryptoException {
        Encryption.keyIv key = new Encryption.keyIv(keylen);
        this.keyMaterial = key.getKeyMaterial();
        this.dataEnc = encryptData(data, keyMaterial);
        this.darcId = writerID;
        this.extraData = "".getBytes();
    }

    /**
     * Overloaded constructor for WriteRequest, but taking a string rather than
     * an array of bytes.
     *
     * @param data     The data for the document, will be encrypted.
     * @param keylen   The key length - 32 bytes is recommended.
     * @param writerID The darc ID where this write request will be stored.
     */
    public WriteRequest(String data, int keylen, DarcId writerID) throws CothorityCryptoException {
        this(data.getBytes(), keylen, writerID);
    }

    /**
     * Create a new document by giving all possible parameters. This call
     * supposes that the data sent here is already encrypted using the
     * keyMaterial and can be decrypted using keyMaterial.
     *
     * @param dataEnc     The ciphertext.
     * @param keyMaterial The symmetric key plus eventually an IV. This will be encrypted under the shared symmetricKey
     *                    of the cothority.
     * @param writerID    The darc ID where this write request will be stored.
     * @param extraData   data that will _not be encrypted_ but will be visible in cleartext on OmniLedger.
     */
    public WriteRequest(byte[] dataEnc, byte[] keyMaterial, DarcId writerID,
                        byte[] extraData) {
        this.dataEnc = dataEnc;
        this.keyMaterial = keyMaterial;
        this.darcId = writerID;
        this.extraData = extraData;
    }

    /**
     * Returns a protobuf-formatted block that can be sent to the cothority
     * for storage on the skipchain. The data and the keyMaterial will be
     * encrypted and stored encrypted on the skipchain. The id, extraData and
     * darc however will be stored in clear.
     *
     * @param X     The aggregate public key of the cothority.
     * @param ltsID The LTS ID, must be created via the RPC call.
     */
    public Calypso.Write toProto(Point X, byte[] ltsID) throws CothorityCryptoException {
        Calypso.Write.Builder write = Calypso.Write.newBuilder();
        write.setExtradata(ByteString.copyFrom(extraData));
        write.setLtsid(ByteString.copyFrom(ltsID));

        try {
            write.setData(ByteString.copyFrom(dataEnc));

            KeyPair randkp = new KeyPair();
            Scalar r = randkp.scalar;
            Point U = randkp.point;
            write.setU(U.toProto());

            Point C = X.mul(r);
            List<Point> Cs = new ArrayList<>();
            for (int from = 0; from < keyMaterial.length; from += Ed25519.pubLen) {
                int to = from + Ed25519.pubLen;
                if (to > keyMaterial.length) {
                    to = keyMaterial.length;
                }
                Point keyEd25519Point = Ed25519Point.embed(Arrays.copyOfRange(keyMaterial, from, to));
                Point Ckey = C.add(keyEd25519Point);
                Cs.add(Ckey);
                write.addCs(Ckey.toProto());
            }

            Point gBar = Ed25519Point.base().mul(new Ed25519Scalar(ltsID));
            Point Ubar = gBar.mul(r);
            write.setUbar(Ubar.toProto());
            KeyPair skp = new KeyPair();
            Scalar s = skp.scalar;
            Point w = skp.point;
            Point wBar = gBar.mul(s);

            MessageDigest hash = MessageDigest.getInstance("SHA-256");
            for (Point c : Cs) {
                hash.update(c.toBytes());
            }
            hash.update(U.toBytes());
            hash.update(Ubar.toBytes());
            hash.update(w.toBytes());
            hash.update(wBar.toBytes());
            hash.update(darcId.getId());
            Scalar E = new Ed25519Scalar(hash.digest());
            write.setE(E.toProto());
            Scalar F = s.add(E.mul(r));
            write.setF(F.toProto());

            return write.build();

        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("Hashing-error: " + e.getMessage());
        }
    }

    /**
     * Get the encrypted data.
     */
    public byte[] getDataEnc() {
        return dataEnc;
    }

    /**
     * Get the symmetric key.
     */
    public byte[] getKeyMaterial() {
        return keyMaterial;
    }

    /**
     * Get the darc ID.
     */
    public DarcId getDarcId() {
        return darcId;
    }

    /**
     * Get the extra data.
     */
    public byte[] getExtraData() {
        return extraData;
    }
}
