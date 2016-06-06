package dcnet

import (
	"testing"

	"github.com/dedis/crypto/nist"
)

func TestSimple(t *testing.T) {
	TestCellCoder(t, nist.NewAES128SHA256P256(), SimpleCoderFactory)
}

func TestOwned(t *testing.T) {
	TestCellCoder(t, nist.NewAES128SHA256P256(), OwnedCoderFactory)
}
