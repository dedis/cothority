package crypto

import (
	"github.com/dedis/crypto/abstract"
	"io/ioutil"
)

func HashFile(suite abstract.Suite, file string) ([]byte, error) {
	hash := suite.Hash()
	hash.Reset()
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	_, err = hash.Write(b)
	if err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
