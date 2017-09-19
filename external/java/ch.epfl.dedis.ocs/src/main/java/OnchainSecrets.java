import com.google.protobuf.ByteString;
import proto.*;

import javax.xml.bind.DatatypeConverter;
import java.util.ArrayList;
import java.util.List;

public class OnchainSecrets {
    public Roster roster;
    public byte[] ocs_id;
    public Crypto.Point X;

    public OnchainSecrets(String group) throws Exception {
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
                X = new Crypto.Point(reply.getX());
                ocs_id = reply.getSB().getHash().toByteArray();
                System.out.println("Initialised OCS");
            } else {
                System.out.println(msg.error);
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            e.printStackTrace();
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
        d.points.add(newAccount.Point);

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
                System.out.println("Updated darc: " + DatatypeConverter.printHexBinary(reply.getSb().getHash().toByteArray()));
            } else {
                System.out.println(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    // returns the shared key of the DKG that must be used to encrypt the symmetric encryption key.
    public Crypto.Point getSharedPublicKey() throws CothorityError {
        try {
            OCSProto.SharedPublicRequest.Builder request =
                    OCSProto.SharedPublicRequest.newBuilder();
            request.setGenesis(ByteString.copyFrom(ocs_id));

            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/SharedPublicRequest", request.build().toByteArray());

            if (msg.ok) {
                OCSProto.SharedPublicReply reply =
                        OCSProto.SharedPublicReply.parseFrom(msg.response);
                System.out.println("Got shared public key");
                return new Crypto.Point(reply.getX());
            } else {
                System.out.println(msg.error);
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    // calling user must be a readers
    // at this point future document reader or seller is not yet known
    // document is created and stored in the system and calling user (readers) becomes owner of the document
    public Document publishDocument(Document doc, Account publisher) throws CothorityError {
        try {

            Document doc_new = new Document(doc);
            doc_new.readers = new Darc();
            doc_new.readers.accounts.add(new Darc.DarcLink(publisher));

            OCSProto.WriteRequest.Builder request =
                    OCSProto.WriteRequest.newBuilder();
            request.setWrite(doc_new.getWrite(X));
            request.setReader(doc_new.readers.getProto());
            request.setOcs(ByteString.copyFrom(ocs_id));
            request.setData(ByteString.copyFrom(doc_new.extra_data));

            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/WriteRequest",
                            request.build().toByteArray());

            if (msg.ok) {
                OCSProto.WriteReply reply =
                        OCSProto.WriteReply.parseFrom(msg.response);
                doc_new.id = reply.getSb().getHash().toByteArray();
                System.out.println("Published document " + Log.toString(doc_new.id));
                return doc_new;
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            e.printStackTrace();
            throw new CothorityError(e.toString());
        }
    }

    public List<Darc> readDarc(byte[] darc_id, Boolean recursive) throws CothorityError {
        OCSProto.ReadDarcRequest.Builder request =
                OCSProto.ReadDarcRequest.newBuilder();
        request.setOcs(ByteString.copyFrom(ocs_id));
        request.setDarcId(ByteString.copyFrom(darc_id));
        request.setRecursive(recursive);
        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/ReadDarcRequest", request.build().toByteArray());

            if (msg.ok) {
                OCSProto.ReadDarcReply reply =
                        OCSProto.ReadDarcReply.parseFrom(msg.response);
                List<Darc> darcs = new ArrayList<>();
                reply.getDarcList().forEach(d -> darcs.add(new Darc(d)));
                System.out.println("Read darcs");
                return darcs;
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    // This adds the consumer to the list of people allowed to make a read-request to the document.
    public void giveReadAccessToDocument(Document d, Account publisher, Account reader) throws CothorityError {
        List<Darc> darcs = readDarc(d.readers.id, false);
        Darc darc = darcs.get(0);
        darc.version++;
        darc.points.add(reader.Point);

        // TODO: sign this new Darc with the readers-account

        OCSProto.EditDarcRequest.Builder request =
                OCSProto.EditDarcRequest.newBuilder();
        request.setOcs(ByteString.copyFrom(ocs_id));
        request.setDarc(darc.getProto());

        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/EditDarcRequest",
                            request.build().toByteArray());

            if (msg.ok) {
                OCSProto.EditDarcReply reply =
                        OCSProto.EditDarcReply.parseFrom(msg.response);
                System.out.println("Read-access granted: " + DatatypeConverter.printHexBinary(reply.getSb().getHash().toByteArray()));
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    public byte[] readRequest(Document d, Account reader) throws CothorityError {
        OCSProto.OCSRead.Builder ocs_read =
                OCSProto.OCSRead.newBuilder();
        ocs_read.setPublic(reader.Point.toProto());
        ocs_read.setDataId(ByteString.copyFrom(d.id));
        ocs_read.setSignature(new Crypto.SchnorrSig(d.id, reader).toProto());

        OCSProto.ReadRequest.Builder request =
                OCSProto.ReadRequest.newBuilder();
        request.setOcs(ByteString.copyFrom(ocs_id));
        request.setRead(ocs_read);

        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/ReadRequest", request.build().toByteArray());

            if (msg.ok) {
                OCSProto.ReadReply reply =
                        OCSProto.ReadReply.parseFrom(msg.response);
                System.out.println("Created a read-request");
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
                return reply;
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            throw new CothorityError(e.toString());
        }
    }

    public DecryptKey decryptKey(byte[] id) throws CothorityError {
        OCSProto.DecryptKeyRequest.Builder request =
                OCSProto.DecryptKeyRequest.newBuilder();
        request.setReadId(ByteString.copyFrom(id));
        try {
            SyncSendMessage msg =
                    new SyncSendMessage(roster.Nodes.get(0), "OnChainSecrets/DecryptKeyRequest",
                            request.build().toByteArray());

            if (msg.ok) {
                OCSProto.DecryptKeyReply reply =
                        OCSProto.DecryptKeyReply.parseFrom(msg.response);
                System.out.println("got decryptKey");
                return new DecryptKey(reply);
            } else {
                throw new CothorityError(msg.error);
            }
        } catch (Exception e) {
            e.printStackTrace();
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
        Document doc = new Document("test", 32);
        return doc;
    }

    public class CothorityError extends Exception {
        public CothorityError(String message) {
            super(message);
        }
    }

}
