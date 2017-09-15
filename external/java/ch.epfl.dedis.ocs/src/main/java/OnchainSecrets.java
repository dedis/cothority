import com.google.protobuf.ByteString;
import net.i2p.crypto.eddsa.EdDSAPublicKey;
import net.i2p.crypto.eddsa.spec.EdDSANamedCurveTable;
import net.i2p.crypto.eddsa.spec.EdDSAPublicKeySpec;
import proto.*;

import java.security.PublicKey;
import java.util.List;

public class OnchainSecrets {
    public Roster roster;
    public byte[] ocs_id;
    public PublicKey X;

    public OnchainSecrets(String group) throws Exception{
        roster = new Roster(group);

        try {
            OCSProto.CreateSkipchainsRequest.Builder request =
                    OCSProto.CreateSkipchainsRequest.newBuilder();
            request.setRoster(roster.getProto());
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/CreateSkipchainsRequest",
                            request.build().toByteArray());

            if (msg.ok) {
                OCSProto.CreateSkipchainsReply reply =
                        OCSProto.CreateSkipchainsReply.parseFrom(msg.response);
                X = Crypto.toPublic(reply.getX());
                ocs_id = reply.getSB().getHash().toByteArray();
            } else {
                System.out.println(msg.error);
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    public OnchainSecrets(String group, byte[] ocs_id) {
        this.ocs_id = ocs_id;
    }

    Boolean ok;

    public Boolean verify() throws CothorityError {
        ok = true;
        roster.Nodes.forEach(n -> {
            try {
                n.GetStatus();
            } catch (Exception e) {
                ok = false;
            }
        });
        return ok;
    }

    public void addAccountToSkipchain(Account admin, Account newAccount) throws CothorityError {
        Darc d = new Darc(newAccount.ID);
        d.public_keys.add(newAccount.Pub);

        OCSProto.EditDarcRequest.Builder request =
                OCSProto.EditDarcRequest.newBuilder();
        request.setDarc(d.getProto());
        request.setOcs(ByteString.copyFrom(ocs_id));

        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/EditDarcRequest", request.build().toByteArray());

            if (msg.ok) {
                OCSProto.EditDarcReply reply =
                        OCSProto.EditDarcReply.parseFrom(msg.response);
//                System.out.println(reply.toString());
            } else {
                System.out.println(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    // returns the shared key of the DKG that must be used to encrypt the symmetric encryption key.
    public PublicKey getSharedPublicKey() throws CothorityError {
        try {
            OCSProto.SharedPublicRequest.Builder request =
                    OCSProto.SharedPublicRequest.newBuilder();
            request.setGenesis(ByteString.copyFrom(ocs_id));

            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/SharedPublicRequest", request.build().toByteArray());

            if (msg.ok) {
                OCSProto.SharedPublicReply reply =
                        OCSProto.SharedPublicReply.parseFrom(msg.response);
                System.out.println(reply.toString());
                return Crypto.toPublic(reply.getX());
            } else {
                System.out.println(msg.error);
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    // calling user must be a publisher
    // at this point future document reader or seller is not yet known
    // document is created and stored in the system and calling user (publisher) become owner of the document
    public Document publishDocument(byte[] encryptedDocument, byte[] descriptionDocument,
                                    byte[] encryptedEncryptionKey,
                                    Account publisher) throws CothorityError {
        try {
            OCSProto.OCSWrite.Builder ocsWrite =
                    OCSProto.OCSWrite.newBuilder();
            ocsWrite.setData(ByteString.copyFrom(encryptedDocument));
            DarcProto.Darc.Builder darc =
                    DarcProto.Darc.newBuilder();
            darc.setId(ByteString.copyFrom(encryptedDocument));
//            darc.setAccounts(0, );
//            darc.setPublicKeys(0, );
            darc.setVersion(0);
            OCSProto.WriteRequest.Builder request =
                    OCSProto.WriteRequest.newBuilder();
            request.setWrite(ocsWrite);
            request.setReader(darc.build());
            request.setOcs(ByteString.copyFrom(ocs_id));
            request.setData(ByteString.copyFrom(descriptionDocument));

            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/Write",
                            request.build().toByteArray());

            if (msg.ok) {
                OCSProto.WriteReply reply =
                        OCSProto.WriteReply.parseFrom(msg.response);
                System.out.println(reply.toString());
                return new Document("test");
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    public List<DarcProto.Darc> readDarc(byte[] darc_id, Boolean recursive) throws CothorityError {
        OCSProto.ReadDarcRequest request =
                OCSProto.ReadDarcRequest.newBuilder().build();
        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/ReadDarc", request.toByteArray());

            if (msg.ok) {
                OCSProto.ReadDarcReply reply =
                        OCSProto.ReadDarcReply.parseFrom(msg.response);
                System.out.println(reply.toString());
                return reply.getDarcList();
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    // This adds the consumer to the list of people allowed to make a read-request to the document.
    public void giveReadAcccessToDocument(Document d, Account reader, Account publisher) throws CothorityError {
        List<DarcProto.Darc> darcs = readDarc(d.darc_id, false);
        DarcProto.Darc.Builder darc = DarcProto.Darc.newBuilder();
        darc.mergeFrom(darcs.get(0));
        darc.setVersion(darc.getVersion() + 1);

        OCSProto.EditDarcRequest.Builder request =
                OCSProto.EditDarcRequest.newBuilder();
        request.setOcs(ByteString.copyFrom(ocs_id));
        request.setDarc(darc);

        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/EditDarc",
                            request.build().toByteArray());

            if (msg.ok) {
                OCSProto.EditDarcReply reply =
                        OCSProto.EditDarcReply.parseFrom(msg.response);
                System.out.println(reply.toString());
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    public byte[] readRequest(Document d, Account reader) throws CothorityError {
        OCSProto.ReadRequest request =
                OCSProto.ReadRequest.newBuilder().build();
        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/Read", request.toByteArray());

            if (msg.ok) {
                OCSProto.ReadReply reply =
                        OCSProto.ReadReply.parseFrom(msg.response);
                System.out.println(reply.toString());
                return reply.getSb().getHash().toByteArray();
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    public SkipBlockProto.SkipBlock getSkipblock(byte[] id) throws CothorityError {
        SkipchainProto.GetSingleBlock request =
                SkipchainProto.GetSingleBlock.newBuilder().build();
        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "Skipchain/GetSingleBlock",
                            request.toByteArray());

            if (msg.ok) {
                SkipBlockProto.SkipBlock reply =
                        SkipBlockProto.SkipBlock.parseFrom(msg.response);
                System.out.println(reply.toString());
                return reply;
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    public DecryptKey decryptKey(byte[] id) throws CothorityError {
        OCSProto.DecryptKeyRequest request =
                OCSProto.DecryptKeyRequest.newBuilder().build();
        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "Skipchain/GetSingleBlock",
                            request.toByteArray());

            if (msg.ok) {
                OCSProto.DecryptKeyReply reply =
                        OCSProto.DecryptKeyReply.parseFrom(msg.response);
                System.out.println(reply.toString());
                DecryptKey dk = new DecryptKey();
                reply.getCsList().forEach(C -> dk.Cs.add(Crypto.toPublic(C)));
                dk.X = Crypto.toPublic(reply.getX());
                dk.XhatEnc = Crypto.toPublic(reply.getXhatEnc());
                return dk;
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    // calling user need DOCUMENT_READ permission
    // get encrypted document - encrypted form will be returned
    public Document readDocument(Document d, Account reader) throws CothorityError {
        byte[] read_id = readRequest(d, reader);
        SkipBlockProto.SkipBlock sb = getSkipblock(d.id);
        DecryptKey dk = decryptKey(read_id);

        // Do some magic
        Document doc = new Document("test");
        return doc;
    }

    public class CothorityError extends Exception {
        public CothorityError(String message) {
            super(message);
        }
    }

}
