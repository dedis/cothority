package ch.epfl.dedis.calypso;

import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.lib.crypto.Ed25519Point;
import ch.epfl.dedis.lib.crypto.Point;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.proto.Calypso;
import com.google.protobuf.InvalidProtocolBufferException;

/**
 * A ReadData is the data that is sent to the calypsoRead contract. It is used to log a read request
 * and must be linked to a corresponding write request.
 */
public class ReadData {
    private Calypso.Read read;

    /**
     * Construct a read request given the ID of the corresponding write request and the reader's public key.
     *
     * @param writeId  the instance of the write request
     * @param readerPk the reader's public key
     */
    public ReadData(InstanceId writeId, Point readerPk) {
        Calypso.Read.Builder b = Calypso.Read.newBuilder();
        b.setWrite(writeId.toByteString());
        b.setXc(readerPk.toProto());
        read = b.build();
    }

    /**
     * Creates a new ReadData from an existing instance.
     *
     * @param inst the instance
     * @throws CothorityNotFoundException if the read request in the instance is corrupt
     */
    public ReadData(Instance inst) throws CothorityNotFoundException {
        if (!inst.getContractId().equals(ReadInstance.ContractId)) {
            throw new CothorityNotFoundException("wrong contract type in instance");
        }
        try {
            read = Calypso.Read.parseFrom(inst.getData());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityNotFoundException("couldn't decode the data: " + e.getMessage());
        }
    }

    /**
     * @return the public key under which the re-encryption will take place.
     */
    public Point getXc() {
        return new Ed25519Point(read.getXc());
    }

    /**
     * @return the instanceId of the corresponding Write Instance.
     */
    public InstanceId getWriteId() {
        return new InstanceId(read.getWrite());
    }

    /**
     * @return the the protobuf representation of the ReadData
     */
    public Calypso.Read toProto() {
        return read;
    }

    /**
     * Takes a byte array as an input to parse into the protobuf representation of ReadData.
     * @param buf the protobuf data
     * @return ReadData
     * @throws InvalidProtocolBufferException if the protobuf data is invalid.
     */
    public static ReadData fromProto(byte[] buf) throws InvalidProtocolBufferException {
        Calypso.Read rd = Calypso.Read.parseFrom(buf);
        return new ReadData(new InstanceId(rd.getWrite()), new Ed25519Point(rd.getXc()));
    }
}
