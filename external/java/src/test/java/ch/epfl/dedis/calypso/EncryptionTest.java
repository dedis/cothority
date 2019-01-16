package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import org.junit.jupiter.api.Test;

import java.security.SecureRandom;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.*;

class EncryptionTest {

    @Test
    void test() throws Exception {
        Random rand = new SecureRandom();
        byte[] data = "abcdefg".getBytes();

        byte[] shortKeyMaterial = new byte[27];
        rand.nextBytes(shortKeyMaterial);
        assertThrows(CothorityCryptoException.class, () -> Encryption.encryptData(data, shortKeyMaterial));

        byte[] keyMaterial = new byte[28];
        rand.nextBytes(keyMaterial);
        assertArrayEquals(data, Encryption.decryptData(Encryption.encryptData(data, keyMaterial), keyMaterial));

        byte[] longKeyMaterial = new byte[29];
        rand.nextBytes(longKeyMaterial);
        assertThrows(CothorityCryptoException.class, () -> Encryption.encryptData(data, longKeyMaterial));
    }
}