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

	// 2nd set of base points
	g2, _ := suite.Point().Pick([]byte("G2"), random.Stream)
	h2, _ := suite.Point().Pick([]byte("H2"), random.Stream)

	// 2nd secret value
	y := suite.Scalar().Pick(random.Stream)

	// Create proof
	p, err := randhound.NewProof(suite, g1, g2, h1, h2)
	if err != nil {
		t.Fatal(err)
	}

	xGxH, core, err := p.Setup(x, y)
	if err != nil {
		t.Fatal(err)
	}

	// Verify proof
	q, _ := randhound.NewProof(suite, g1, g2, h1, h2)
	q.SetCore(core)

	failed, err := q.Verify(xGxH...)
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

	// 2nd set of base points
	g2, _ := suite.Point().Pick([]byte("G2"), random.Stream)
	h2, _ := suite.Point().Pick([]byte("H2"), random.Stream)

	// 2nd secret value
	y := suite.Scalar().Pick(random.Stream)

	// Create proof
	p, err := randhound.NewProof(suite, g1, g2, h1, h2)
	if err != nil {
		t.Fatal(err)
	}

	xGxH, core, err := p.SetupCollective(x, y)
	if err != nil {
		t.Fatal(err)
	}

	// Verify proof
	q, _ := randhound.NewProof(suite, g1, g2, h1, h2)
	q.SetCore(core)

	failed, err := q.Verify(xGxH...)
	if err != nil {
		t.Fatal(err)
	}

	if len(failed) != 0 {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", failed)
	}
}
