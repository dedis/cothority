package ch.epfl.dedis.lib.calypso;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.omniledger.Proof;
import ch.epfl.dedis.proto.Calypso;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;

/**
 * CalypsoRPC is the entry point for all the RPC calls to the Calypso service, which acts as the secret-management cothority.
 */
public class CalypsoRPC {
    /**
     * Create a long-term-secret (LTS) and retrieve its configuration.
     *
     * @param roster  The roster that holds the LTS.
     * @param genesis The genesis block.
     * @return The LTS configuration that is needed to execute the write contract.
     * @throws CothorityCommunicationException
     */
    public static CreateLTSReply createLTS(Roster roster, SkipblockId genesis) throws CothorityCommunicationException {
        Calypso.CreateLTS.Builder b = Calypso.CreateLTS.newBuilder();
        b.setRoster(roster.toProto());
        b.setOlid(ByteString.copyFrom(genesis.getId()));

        ByteString msg = roster.sendMessage("calypso/CreateLTS", b.build());

        try {
            Calypso.CreateLTSReply resp = Calypso.CreateLTSReply.parseFrom(msg);
            return new CreateLTSReply(resp);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Ask the secret-manageemnt cothority for the decryption shares.
     *
     * @param readProof  The proof of the read request.
     * @param writeProof The proof of the write request.
     * @param roster     The roster that holds the shared secret.
     * @return All the decryption shares that can be used to reconstruct the decryption key.
     * @throws CothorityCommunicationException
     */
    public static DecryptKeyReply tryDecrypt(Proof readProof, Proof writeProof, Roster roster) throws CothorityCommunicationException {
        Calypso.DecryptKey.Builder b = Calypso.DecryptKey.newBuilder();
        b.setRead(readProof.toProto());
        b.setWrite(writeProof.toProto());

        ByteString msg = roster.sendMessage("calypso/DecryptKey", b.build());

        try {
            Calypso.DecryptKeyReply resp = Calypso.DecryptKeyReply.parseFrom(msg);
            return new DecryptKeyReply(resp);
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }
}
