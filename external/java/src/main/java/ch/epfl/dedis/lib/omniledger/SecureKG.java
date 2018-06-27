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
                "  Public = \"13dc1a4f714422e7952cef38efd527925341efaa3a992398cb52fa3e0e6dd2b8\"\n" +
                "  Description = \"Conode_1\"\n" +
                "[[servers]]\n" +
                "  Address = \"tls://securekg.dedis.ch:18004\"\n" +
                "  Suite = \"Ed25519\"\n" +
                "  Public = \"705f2877119a39f366ea53f381e37234f9678dee5f17c9f20b11df7c6cdc0e64\"\n" +
                "  Description = \"Conode_2\"\n" +
                "[[servers]]\n" +
                "  Address = \"tls://securekg.dedis.ch:18006\"\n" +
                "  Suite = \"Ed25519\"\n" +
                "  Public = \"1084f8f919112931b18a545e14e4cb668ba0b6d4884f64b463fe3fa4493b8f0e\"\n" +
                "  Description = \"Conode_3\"\n");
    }

    /**
     * Gets the genesis skipblock ID of an existing omniledger service.
     * @return the genesis skipblock ID
     */
    public static SkipblockId getSkipchainId() throws CothorityCryptoException {
        return new SkipblockId(DatatypeConverter.parseHexBinary("77671f623cb3471b756f5ff88b66b1b10c8cbfee2740fba52368565d2b48c7a9"));
    }

    /**
     * Gets the signer that has "invoke:eventlog" and "spawn:eventlog" permissions.
     */
    public static Signer getSigner() {
        // output of "el create --keys"
        // Identity: ed25519:c903f5a3c6388254fb401184ce46a8b3db544b820bbe9eebb7c2c0a9bdfc07a3
        // export PRIVATE_KEY=e70318856e0e9ced0840db6fff6f9296f52e36dc262dc388fa443bf1c6a07e0a
        return new SignerEd25519(DatatypeConverter.parseHexBinary("e70318856e0e9ced0840db6fff6f9296f52e36dc262dc388fa443bf1c6a07e0a"));
    }

    /**
     * Gets the darc ID that has the "invoke:eventlog" and "spawn:eventlog" rules.
     * @return the darc ID
     */
    public static DarcId getDarcId() throws CothorityCryptoException {
        return new DarcId(DatatypeConverter.parseHexBinary("f058943d96072c13a09031dcdec9e99c2972ec1cc9b1e7979ceb988d1978c580"));
    }

    /**
     * Gets the eventlog instance ID.
     * @return the instance ID.
     */
    public static InstanceId getEventlogId() throws CothorityCryptoException {
        // output of ./el create
        //export EL=f058943d96072c13a09031dcdec9e99c2972ec1cc9b1e7979ceb988d1978c580e36393071922a3ff58ba58ffc0b728967d7c25267ace6755d3f1e69e038f4de0
        return new InstanceId(DatatypeConverter.parseHexBinary("f058943d96072c13a09031dcdec9e99c2972ec1cc9b1e7979ceb988d1978c580e36393071922a3ff58ba58ffc0b728967d7c25267ace6755d3f1e69e038f4de0"));
    }

    /**
     * Get the pre-configured omniledger RPC.
     * @return the omniledger RPC object
     */
    public static OmniledgerRPC getOmniledgerRPC() throws CothorityException {
        return new OmniledgerRPC(getRoster(), getSkipchainId());
    }

}
