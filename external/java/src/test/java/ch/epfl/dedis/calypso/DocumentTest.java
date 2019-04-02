package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.Hex;
import ch.epfl.dedis.lib.darc.DarcId;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

class DocumentTest {

    @Test
    void equals() {
        DarcId id = new DarcId(Hex.parseHexBinary("aaaaaaaabbbbbbbbccccccccddddddddaaaaaaaabbbbbbbbccccccccdddddddd"));

        Document doc1 = new Document("some data".getBytes(), "key material".getBytes(), "extra data".getBytes(), id);
        Document doc2 = new Document("some data".getBytes(), "key material".getBytes(), "extra data".getBytes(), id);
        assertEquals(doc1, doc2);

        doc1 = new Document("some data".getBytes(), "key material".getBytes(), "extra data".getBytes(), id);
        doc2 = new Document("some data".getBytes(), "key material".getBytes(), null, id);
        assertNotEquals(doc1, doc2);

        doc1 = new Document("some data".getBytes(), "key material".getBytes(), null, id);
        doc2 = new Document("some data".getBytes(), "key material".getBytes(), null, id);
        assertEquals(doc1, doc2);
    }
}