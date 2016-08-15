package randhound_test

import (
	"testing"

	"github.com/dedis/cothority/protocols/randhound"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
)

func TestProof(t *testing.T) {

	suite := edwards.NewAES128SHA256Ed25519(false)

	x := suite.Scalar().Pick(random.Stream)
	g, _ := suite.Point().Pick([]byte("G"), random.Stream)
	h, _ := suite.Point().Pick([]byte("H"), random.Stream)

	gx := suite.Point().Mul(g, x)
	hx := suite.Point().Mul(h, x)

	p := randhound.NewProof(suite, g, h)
	p.Setup(x)

	if !p.Verify(gx, hx) {
		t.Fatal("Verification of discrete logarithm proof failed")
	}

}
