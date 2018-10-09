package ch.epfl.dedis.lib.darc;

import java.util.Arrays;

public class SignerFactory {
    public static final byte IDEd25519 = 1;
    public static final byte Keycard = 2;

    /**
     * Returns the signer corresponding to the data.
     *
     * @param data the signer in serialised form
     * @return the new Signer
     * @throws RuntimeException if invalid data is passed in
     */
    public static Signer New(byte[] data) throws RuntimeException{
        switch (data[0]){
            case IDEd25519:
                return new SignerEd25519(Arrays.copyOfRange(data, 1, data.length));
            case Keycard:
                throw new IllegalStateException("Sorry but keycard signer can not be serialised/deserialised");
            default:
                throw new RuntimeException("invalid data");
        }
    }
}
