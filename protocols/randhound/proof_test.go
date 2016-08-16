package randhound_test

import (
	"testing"

	"github.com/dedis/cothority/protocols/randhound"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
)

func TestProof(t *testing.T) {

	suite := edwards.NewAES128SHA256Ed25519(false)

	// 1st set of base points
	g1, _ := suite.Point().Pick([]byte("G1"), random.Stream)
	h1, _ := suite.Point().Pick([]byte("H1"), random.Stream)

	// 1st secret value
	x := suite.Scalar().Pick(random.Stream)

	// 1st set of public values
	g1x := suite.Point().Mul(g1, x)
	h1x := suite.Point().Mul(h1, x)

	// 2nd set of base points
	g2, _ := suite.Point().Pick([]byte("G2"), random.Stream)
	h2, _ := suite.Point().Pick([]byte("H2"), random.Stream)

	// 2nd secret value
	y := suite.Scalar().Pick(random.Stream)

	// 2nd set of public values
	g2y := suite.Point().Mul(g2, y)
	h2y := suite.Point().Mul(h2, y)

	p, _ := randhound.NewProof(suite, g1, h1, g2, h2)
	p.Setup(x, y)

	failed, err := p.Verify(g1x, h1x, g2y, h2y)
	if err != nil {
		t.Fatal(err)
	}

	if len(failed) != 0 {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", failed)
	}

}

func TestProofCollective(t *testing.T) {

	suite := edwards.NewAES128SHA256Ed25519(false)

	// 1st set of base points
	g1, _ := suite.Point().Pick([]byte("G1"), random.Stream)
	h1, _ := suite.Point().Pick([]byte("H1"), random.Stream)

	// 1st secret value
	x := suite.Scalar().Pick(random.Stream)

	// 1st set of public values
	g1x := suite.Point().Mul(g1, x)
	h1x := suite.Point().Mul(h1, x)

	// 2nd set of base points
	g2, _ := suite.Point().Pick([]byte("G2"), random.Stream)
	h2, _ := suite.Point().Pick([]byte("H2"), random.Stream)

	// 2nd secret value
	y := suite.Scalar().Pick(random.Stream)

	// 2nd set of public values
	g2y := suite.Point().Mul(g2, y)
	h2y := suite.Point().Mul(h2, y)

	p, _ := randhound.NewProof(suite, g1, h1, g2, h2)
	p.SetupCollective(x, y)

	failed, err := p.Verify(g1x, h1x, g2y, h2y)
	if err != nil {
		t.Fatal(err)
	}

	if len(failed) != 0 {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", failed)
	}
}
