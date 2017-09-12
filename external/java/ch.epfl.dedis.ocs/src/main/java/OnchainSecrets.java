import java.security.PublicKey;

public class OnchainSecrets {
    private Roster roster;

    public OnchainSecrets(String group) {
        this.roster = new Roster(group);
    }

    public Boolean verify() throws CothorityError{
        this.roster.Nodes.forEach(n -> n.testNode());
    }

    public void addAccountToSkipchain(Account admin, Account newAccount) throws CothorityError {

    }

    // returns the shared key of the DKG that must be used to encrypt the symmetric encryption key.
    public PublicKey getSharedPublicKey() throws CothorityError {
        return null;
    }

    // calling user must be a publisher
    // at this point future document reader or seller is not yet known
    // document is created and stored in the system and calling user (publisher) become owner of the document
    public Document publishDocument(byte[] encryptedDocument, byte[] encryptedEncryptionKey,
                                    Account publisher) throws CothorityError {
        return null;
    }

    // This adds the consumer to the list of people allowed to make a read-request to the document.
    public void giveReadAcccessToDocument(Document d, Account reader, Account publisher) throws CothorityError {

    }

    // calling user need DOCUMENT_READ permission
    // get encrypted document - encrypted form will be returned
    public Document readDocument(Document d, Account reader) throws CothorityError {
        return null;
    }

    public class CothorityError extends Exception {
    }
}
