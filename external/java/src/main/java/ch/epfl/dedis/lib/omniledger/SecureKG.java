package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import ch.epfl.dedis.lib.omniledger.darc.Signer;
import ch.epfl.dedis.lib.omniledger.darc.SignerEd25519;

import javax.xml.bind.DatatypeConverter;

public final class SecureKG {
    /**
     * Gets the roster of the secure KG server.
     * @return the roster
     */
    public static Roster getRoster() {
        return Roster.FromToml("[[servers]]\n" +
                "  Address = \"tls://securekg.dedis.ch:18002\"\n" +
                "  Suite = \"Ed25519\"\n" +
                "  Public = \"fcf9492de37b1115637206f3bc6ace77e6d8e7ae29b0b40dd7a0f18bdd2eb7db\"\n" +
                "  Description = \"Conode_1\"\n" +
                "[[servers]]\n" +
                "  Address = \"tls://securekg.dedis.ch:18004\"\n" +
                "  Suite = \"Ed25519\"\n" +
                "  Public = \"11c7c22112328f151e6d853f47bb3b404bbcbcaf1bc1ab30a25eedd3d7682324\"\n" +
                "  Description = \"Conode_2\"\n" +
                "[[servers]]\n" +
                "  Address = \"tls://securekg.dedis.ch:18006\"\n" +
                "  Suite = \"Ed25519\"\n" +
                "  Public = \"2aec81a7d16891c4806fb73f7e247eca311415fc21a54193e84e6c73f6df9b3f\"\n" +
                "  Description = \"Conode_3\"");
    }

    /**
     * Gets the genesis skipblock ID of an existing omniledger service.
     * @return the genesis skipblock ID
     */
    public static SkipblockId getSkipchainId() throws CothorityCryptoException {
        return new SkipblockId(DatatypeConverter.parseHexBinary("30cdbf5b5acc0e4dd227d8ee7fa845d419f79d43e824a3523a30d921a895838e"));
    }

    /**
     * Gets the signer that has "invoke:eventlog" and "spawn:eventlog" permissions.
     */
    public static Signer getSigner() {
        // public is 73cfc340c552145a8d8619cbbc0e788379c7a261764afd1d81fa0f971442140f
        return new SignerEd25519(DatatypeConverter.parseHexBinary("022f59c145d4863d72cfd541628da08f4c907fb34f921dfeca8a1f35b3c0310a"));
    }

    /**
     * Gets the darc ID that has the "invoke:eventlog" and "spawn:eventlog" rules.
     * @return the darc ID
     */
    public static DarcId getDarcId() throws CothorityCryptoException {
        return new DarcId(DatatypeConverter.parseHexBinary("b880482b89fba327e36fbb27b482d883a9e4354ae72078b37592ae86c8219580"));
    }

    /**
     * Gets the eventlog instance ID.
     * @return the instance ID.
     */
    public static InstanceId getEventlogId() throws CothorityCryptoException {
        return new InstanceId(DatatypeConverter.parseHexBinary("b880482b89fba327e36fbb27b482d883a9e4354ae72078b37592ae86c8219580eeca4910a865c9fc246d1933a79ef1b44f776029b213ad6541f0f5b5025da2f1"));
    }

    /**
     * Get the pre-configured omniledger RPC.
     * @return the omniledger RPC object
     */
    public static OmniledgerRPC getOmniledgerRPC() throws CothorityException {
        return new OmniledgerRPC(getRoster(), getSkipchainId());
    }

}
