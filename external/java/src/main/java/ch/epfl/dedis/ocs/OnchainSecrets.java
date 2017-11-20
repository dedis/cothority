package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.darc.*;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.OCSProto;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.List;

/**
 * OnchainSecrets interfaces the OnchainSecretsRPC class and offers convenience methods for
 * easier handling.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.3 17/11/07
 */
public class OnchainSecrets extends OnchainSecretsRPC {
    private final Logger logger = LoggerFactory.getLogger(OnchainSecrets.class);

    /**
     * Creates a new OnchainSecrets class that attaches to an existing skipchain.
     *
     * @param roster
     * @param ocsID
     */
    public OnchainSecrets(Roster roster, SkipblockId ocsID) throws CothorityCommunicationException, CothorityCryptoException {
        super(roster, ocsID);
    }

    /**
     * Creates a new OnchainSecrets class and creates a new skipchain.
     *
     * @param roster
     * @param admin
     */
    public OnchainSecrets(Roster roster, Darc admin) throws CothorityCommunicationException, CothorityCryptoException {
        super(roster, admin);
    }

    /**
     * Convenience method to pass a signer as identity and get the darc-path used in signatures.
     *
     * @param base     the darc that should be taken as reference to build the darc path.
     * @param identity which identity wants to sign using that darc. The search algorithm does
     *                 a breadth-first search of this identity in the darc.
     * @param role     the role to search for. An identity might be stored as user AND as an owner,
     *                 so we cannot rely on the first occurrence but need to indicate which role the
     *                 identity should have.
     * @return if the darc is stored in the skipchain, the list of all darcs leading up to the identity
     * is returned.
     * @throws CothorityCommunicationException
     * @throws CothorityCryptoException
     */
    public SignaturePath getDarcPath(DarcId base, Signer identity, int role) throws CothorityCommunicationException,
            CothorityCryptoException {
        return getDarcPath(base, IdentityFactory.New(identity), role);
    }

    /**
     * Adds a new identity to an existing darc under the given role. The darc must already be in its
     * latest version. After the new darc is created, the darc is stored on the skipchain and returned
     * as a value.
     *
     * @param darc     the latest version of the darc where an identity should be added to.
     * @param identity the identity to be added to the darc.
     * @param signer   must be an owner of the darc.
     * @param role     the role the new identity should have in the darc.
     * @return the new darc
     * @throws CothorityCommunicationException if the new darc could not be stored on the skipchain
     * @throws CothorityCryptoException        if the signer could not sign the darc.
     */
    public Darc addIdentityToDarc(Darc darc, Identity identity, Signer signer, int role) throws CothorityCommunicationException, CothorityCryptoException {
        Darc newDarc = darc.copy();
        switch (role) {
            case SignaturePath.USER:
                newDarc.addUser(identity);
                break;
            case SignaturePath.OWNER:
                newDarc.addOwner(identity);
                break;
            default:

        }
        SignaturePath path = getDarcPath(darc.getId(), signer, SignaturePath.OWNER);
        newDarc.setEvolution(darc, path, signer);
        updateDarc(newDarc);
        return newDarc;
    }

    /**
     * Overloaded method for convenience in case the identity is only available as a signer.
     *
     * @param darc     the latest version of the darc where an identity should be added to.
     * @param identity the identity to be added to the darc.
     * @param signer   must be an owner of the darc.
     * @param role     the role the new identity should have in the darc.
     * @return the new darc
     * @throws CothorityCommunicationException if the new darc could not be stored on the skipchain
     * @throws CothorityCryptoException        if the signer could not sign the darc.
     */
    public Darc addIdentityToDarc(Darc darc, Signer identity, Signer signer, int role) throws CothorityCommunicationException, CothorityCryptoException {
        Identity newI = IdentityFactory.New(identity);
        return addIdentityToDarc(darc, newI, signer, role);
    }

    /**
     * Instead of giving a darc, this method will search for an existing darc given its id on the
     * skipchain.
     *
     * @param id       the id of the latest version of the darc where an identity should be added to.
     * @param identity the identity to be added to the darc.
     * @param signer   must be an owner of the darc.
     * @param role     the role the new identity should have in the darc.
     * @return the new darc
     * @throws CothorityCommunicationException if the new darc could not be stored on the skipchain
     * @throws CothorityCryptoException        if the signer could not sign the darc.
     */
    public Darc addIdentityToDarc(DarcId id, Signer identity, Signer signer, int role) throws CothorityCommunicationException, CothorityCryptoException {
        List<Darc> darcs = getLatestDarc(id);
        return addIdentityToDarc(darcs.get(darcs.size() - 1), identity, signer, role);
    }

    /**
     * Publishes a document given the Document and the writer with write-authorization. The document already
     * needs to be prepared with encrypted data and the keymaterial set up correctly.
     *
     * @param doc    a prepared document to be stored on the skipchain
     * @param writer one of the authorized writers to the skipchain
     * @return WriteRequest with the given getId
     * @throws CothorityCryptoException        if the writer could not sign the request
     * @throws CothorityCommunicationException if the request could not be stored on the skipchain
     */
    public WriteRequest publishDocument(Document doc, Signer writer) throws CothorityCryptoException, CothorityCommunicationException{
        WriteRequest wr = doc.getWriteRequest();
        DarcSignature sig = wr.getSignature(this,  writer);
        return createWriteRequest(wr, sig);
    }

    /**
     * Creates a read-request, if successful fetches the document from the skipchain and decodes the
     * keymaterial.
     *
     * @param wrId   the id of the writerequest on the skipchain
     * @param reader a reader with access to the document
     * @return the document with decrypted keymaterial. The data-part still needs to be
     * encrypted by the user.
     * @throws CothorityCryptoException        if the signer could not sign the request
     * @throws CothorityCommunicationException if the request could not be stored on the skipchain
     */
    public Document getDocument(WriteRequestId wrId, Signer reader) throws CothorityCryptoException, CothorityCommunicationException {
        OCSProto.Write document = getWrite(wrId);
        Darc readerDarc = new Darc(document.getReader());
        SignaturePath path = getDarcPath(readerDarc.getId(), IdentityFactory.New(reader), SignaturePath.USER);

        for (Darc d : path.getDarcs()) {
            logger.debug("Path: " + d.toString());
        }

        ReadRequestId rrid = createReadRequest(new ReadRequest(this, wrId, reader));
        DecryptKey dk = getDecryptionKey(rrid);
        OCSProto.Write write = getWrite(wrId);
        byte[] keyMaterial = dk.getKeyMaterial(write, reader);
        return new Document(write.getData().toByteArray(), keyMaterial, write.getExtradata().toByteArray(),
                readerDarc, wrId);
    }
}
