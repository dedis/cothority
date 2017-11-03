package ch.epfl.dedis.lib.darc;

import java.util.Arrays;

public class SignerFactory {
    public static final byte IDEd25519 = 1;

    /**
     * Returns the signer corresponding to the data.
     *
     * @param data
     * @return
     */
    public static Signer New(byte[] data) throws Exception{
        switch (data[0]){
            case IDEd25519:
                return new Ed25519Signer(Arrays.copyOfRange(data, 1, data.length));
            default:
                throw new Exception("invalid data");
        }
    }
}
