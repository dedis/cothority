package ch.epfl.dedis.lib;

import ch.epfl.dedis.lib.exception.CothorityException;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public class SkipBlockTest {
    @Test
    void testHash() throws CothorityException {
        // See skipchain/struct_test.go for where these hex values come from.
        byte[] canned = Hex.parseHexBinary("08001008180020003a004201314a94010a106bc1027de8ef542e8b09219c287b2fde12560a2865642e706f696e7400000000000000000000000000000000000000000000000000000000000000001a103809e37975a45b4a865899668d645d9522147463703a2f2f3132372e302e302e313a323030302a003a001a2865642e706f696e74000000000000000000000000000000000000000000000000000000000000000052201304bd5ecad8d54a2fd7b81a8864f698966308104b20780b634c4b237b8438236200");
        SkipBlock sb = new SkipBlock(canned);
        assertEquals(sb.getData().length, 1);
        assertEquals(sb.getData()[0], 49); // "1"
        assertEquals(sb.getHeight(), 4);
        assertEquals(sb.getRoster().getNodes().size(), 1);
        byte[] expectedHash = Hex.parseHexBinary("1304bd5ecad8d54a2fd7b81a8864f698966308104b20780b634c4b237b843823");
        assertArrayEquals(expectedHash, sb.getHash());
    }
}
