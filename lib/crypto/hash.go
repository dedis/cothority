package crypto

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"io"
	"os"
)

// Hash simply returns the Hash of the slice of bytes given
func Hash(suite abstract.Suite, buff []byte) ([]byte, error) {
	h := suite.Hash()
	_, err := h.Write(buff)
	return h.Sum(nil), err
}

// Size of the chunk  used to read a stream
const DefaultChunkSize = 1024

// Hash a stream reading from it by chunks of DefaultChunkSize size.
func hashStream(suite abstract.Suite, stream io.Reader, size int) ([]byte, error) {
	b := make([]byte, size)
	hash := suite.Hash()
	for {
		n, errRead := stream.Read(b)
		dbg.Lvl4("Read", n, "bytes of", size)
		_, err := hash.Write(b[:n])
		if err != nil {
			return nil, err
		}
		if errRead == io.EOF {
			break
		}
	}
	return hash.Sum(nil), nil
}

// HashStream returns the hash of the stream reading from it chunk by chunk of
// size DefaultChunkSize
func HashStream(suite abstract.Suite, stream io.Reader) ([]byte, error) {
	return hashStream(suite, stream, DefaultChunkSize)
}

// Hash a stream using chunks of size
func HashStreamChunk(suite abstract.Suite, stream io.Reader, chunkSize int) ([]byte, error) {
	if chunkSize < 1 {
		return nil, errors.New("Wront chunksize value")
	}
	return hashStream(suite, stream, chunkSize)
}

// HashFile will hash the file using the streaming approach with
// DefaultChunkSize size of chunks
func HashFile(suite abstract.Suite, file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return HashStream(suite, f)

}

// HashFileChunk is similar to HashFile but using a chunkSize size of chunks for
// reading.
func HashFileChunk(suite abstract.Suite, file string, chunkSize int) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return HashStreamChunk(suite, f, chunkSize)
}
