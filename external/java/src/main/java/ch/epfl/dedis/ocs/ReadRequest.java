package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.OCSProto;
import com.google.protobuf.ByteString;

/**
 * This represents a read-request as it has to be sent to the skipchain. It can communicate with the
 * conodes to get all necessary information from the writerequest-id, or create a read-request if the
 * information is stored.
 */
public class ReadRequest {
    private WriteRequestId wrId;
    private DarcSignature signature;

    /**
     * This fetches the write-request and the path for the reader to sign. If the reader is
     * not present in any of the darcs of the write-request, an exception will be created.
     *
     * @param ocs a pointer to an initialized OnchainSecrets for retrieving the write request and
     *            the path
     * @param wrId the write request the reader wants to access
     * @param reader a reader with access to the document
     * @throws CothorityCommunicationException
     * @throws CothorityCryptoException
     */
    public ReadRequest(OnchainSecretsRPC ocs, WriteRequestId wrId, Signer reader) throws CothorityCommunicationException,
            CothorityCryptoException{
        OCSProto.Write wr = ocs.getWrite(wrId);
        this.wrId = wrId;
        Darc readDarc = new Darc(wr.getReader());
        Identity readerId = IdentityFactory.New(reader);
        SignaturePath path = ocs.getDarcPath(readDarc.getId(), readerId, SignaturePath.USER);
        this.signature = new DarcSignature(wrId.getId(), path, reader);
    }

    public ReadRequest(WriteRequestId wrId, DarcSignature signature){
        this.wrId = wrId;
        this.signature = signature;
    }

    /**
     * Return the protobuf-representation of the ReadRequest.
     * @return
     */
    public OCSProto.Read ToProto(){
        OCSProto.Read.Builder ocsRead =
                OCSProto.Read.newBuilder();
        ocsRead.setDataid(ByteString.copyFrom(wrId.getId()));
        ocsRead.setSignature(signature.toProto());
        return ocsRead.build();
    }
}
