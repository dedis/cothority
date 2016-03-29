package crypto

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"os"
)

func HashFileSlice(suite abstract.Suite, file string, size int) ([]byte, error) {
	if size == 0 {
		return nil, errors.New("Cannot read 0 bytes")
	}
	hash := suite.Hash()
	hash.Reset()

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	b := make([]byte, size)
	n := size
	for n == size {
		n, err = f.Read(b)
		dbg.Lvl3("Read", n, "bytes of", size)
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}
	return hash.Sum(nil), nil
}

func HashFile(suite abstract.Suite, file string) ([]byte, error) {
	b, err := HashFileSlice(suite, file, 1024)
	return b, err
}
