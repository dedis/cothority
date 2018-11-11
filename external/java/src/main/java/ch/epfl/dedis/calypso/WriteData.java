package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.lib.crypto.*;
import ch.epfl.dedis.lib.darc.DarcId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.proto.Calypso;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * A WriteData is the data that is sent to the calypsoWrite contract store a write request with the encrypted document.
 * Stored on BzyCoin, it will have the following fields:
 * <p>
 * - EncData - the encrypted data, should be smaller than 8MB
 * - U, Ubar, E, F, Cs - the symmetric key used to encrypt the data, itself encrypted to the Long Term Secret key
 * - ExtraData - plain text data that is stored as-is on the ledger
 * - LTSID - the Long Term Secret ID used to encrypt the data
 */
public class WriteData {
    private Calypso.Write write;

    /**
     * Create a new document by giving all possible parameters. This call
     * supposes that the data sent here is already encrypted using the
     * keyMaterial and can be decrypted using keyMaterial.
     *
     * @param lts         Long Term Secret parameters
     * @param dataEnc     The ciphertext which will be stored _as is_ on ByzCoin.
     * @param keyMaterial The symmetric key plus eventually an IV. This will be encrypted under the shared symmetricKey
     *                    of the cothority.
     * @param extraData   data that will _not be encrypted_ but will be visible in cleartext on ByzCoin.
     * @param publisher   The darc with a rule for calypsoWrite and calypsoRead.
     * @throws CothorityException if something went wrong
     */
    public WriteData(LTS lts, byte[] dataEnc, byte[] keyMaterial, byte[] extraData, DarcId publisher) throws CothorityException {
        if (dataEnc.length > 8000000) {
            throw new CothorityException("data length too long");
        }
        Calypso.Write.Builder wr = Calypso.Write.newBuilder();
        wr.setData(ByteString.copyFrom(dataEnc));
        if (extraData != null) {
            wr.setExtradata(ByteString.copyFrom(extraData));
        }
        wr.setLtsid(lts.getLtsId().toProto());
        encryptKey(wr, lts, keyMaterial, publisher);
        write = wr.build();
    }

    /**
     * Private constructor if we know Calypso.Write
     *
     * @param w Calypso.Write
     */
    private WriteData(Calypso.Write w) {
        write = w;
    }

    /**
     * Recreates a WriteData from an instanceid.
     *
     * @param bc a running Byzcoin service
     * @param id an instanceId of a WriteInstance
     * @throws CothorityNotFoundException if the requested instance cannot be found
     * @throws CothorityCommunicationException if something went wrong
     * @return the new WriteData
     */
    public static WriteData fromByzcoin(ByzCoinRPC bc, InstanceId id) throws CothorityNotFoundException, CothorityCommunicationException {
        return WriteData.fromInstance(Instance.fromByzcoin(bc, id));
    }

    /**
     * Recreates a WriteData from an instance.
     *
     * @param inst an instance representing a WriteData
     * @return WriteData
     * @throws CothorityNotFoundException if the requested instance cannot be found
     */
    public static WriteData fromInstance(Instance inst) throws CothorityNotFoundException {
        if (!inst.getContractId().equals(WriteInstance.ContractId)) {
            throw new CothorityNotFoundException("Wrong contract in instance");
        }
        try {
            return new WriteData(Calypso.Write.parseFrom(inst.getData()));
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityNotFoundException("couldn't parse protobuffer for writeData: " + e.getMessage());
        }
    }

    /**
     * Encrypts the key material and stores it in the given Write.Builder.
     *
     * @param wr          the Write.Builder where the encrypted key will be stored
     * @param lts         the Long Term Secret to use
     * @param keyMaterial what should be threshold encrypted in the blockchain
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    private void encryptKey(Calypso.Write.Builder wr, LTS lts, byte[] keyMaterial, DarcId darcId) throws CothorityCryptoException {
        try {
            KeyPair randkp = new KeyPair();
            Scalar r = randkp.scalar;
            Point U = randkp.point;
            wr.setU(U.toProto());

            Point C = lts.getX().mul(r);
            List<Point> Cs = new ArrayList<>();
            for (int from = 0; from < keyMaterial.length; from += Ed25519.pubLen) {
                int to = from + Ed25519.pubLen;
                if (to > keyMaterial.length) {
                    to = keyMaterial.length;
                }
                Point keyEd25519Point = Ed25519Point.embed(Arrays.copyOfRange(keyMaterial, from, to));
                Point Ckey = C.add(keyEd25519Point);
                Cs.add(Ckey);
                wr.addCs(Ckey.toProto());
            }

            Point gBar = Ed25519Point.base().mul(new Ed25519Scalar(lts.getLtsId().getId()));
            Point Ubar = gBar.mul(r);
            wr.setUbar(Ubar.toProto());
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
            wr.setE(E.toProto());
            Scalar F = s.add(E.mul(r));
            wr.setF(F.toProto());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("Hashing-error: " + e.getMessage());
        }
    }

    /**
     * Get the encrypted data.
     * @return the encrypted data
     */
    public byte[] getDataEnc() {
        return write.getData().toByteArray();
    }

    /**
     * Get the extra data.
     * @return the extra data
     */
    public byte[] getExtraData() {
        return write.getExtradata().toByteArray();
    }

    public Calypso.Write getWrite() {
        return write;
    }
}
