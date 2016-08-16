package randhound_test

import (
	"fmt"
	"testing"

	"github.com/dedis/cothority/protocols/randhound"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
)

func TestProof(t *testing.T) {

	suite := edwards.NewAES128SHA256Ed25519(false)

	x := suite.Scalar().Pick(random.Stream)
	y := suite.Scalar().Pick(random.Stream)
	g1, _ := suite.Point().Pick([]byte("G1"), random.Stream)
	h1, _ := suite.Point().Pick([]byte("H1"), random.Stream)
	g2, _ := suite.Point().Pick([]byte("G2"), random.Stream)
	h2, _ := suite.Point().Pick([]byte("H2"), random.Stream)

	g1x := suite.Point().Mul(g1, x)
	h1x := suite.Point().Mul(h1, x)

	g2y := suite.Point().Mul(g2, y)
	//h2y := suite.Point().Mul(h2, y)

	p, _ := randhound.NewProof(suite, g1, h1, g2, h2)
	p.Setup(x, y)

	failed, err := p.Verify(g1x, h1x, g2y, g2y)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(failed)

	if len(failed) != 0 {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", failed)
	}

}
