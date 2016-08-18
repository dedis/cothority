package randhound_test

import (
	"testing"

	"github.com/dedis/cothority/protocols/randhound"
	"github.com/dedis/crypto/abstract"
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
	g := []abstract.Point{g1, g2}
	h := []abstract.Point{h1, h2}
	p, err := randhound.NewProof(suite, g, h, nil)
	if err != nil {
		t.Fatal(err)
	}

	xG, xH, core, err := p.Setup(x, y)
	if err != nil {
		t.Fatal(err)
	}

	// Verify proof
	q, err := randhound.NewProof(suite, g, h, core)
	if err != nil {
		t.Fatal(err)
	}
	//q.SetCore(core)

	f, err := q.Verify(xG, xH)
	if err != nil {
		t.Fatal(err)
	}

	if len(f) != 0 {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", f)
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
	g := []abstract.Point{g1, g2}
	h := []abstract.Point{h1, h2}
	p, err := randhound.NewProof(suite, g, h, nil)
	if err != nil {
		t.Fatal(err)
	}

	xG, xH, core, err := p.SetupCollective(x, y)
	if err != nil {
		t.Fatal(err)
	}

	// Verify proof
	q, _ := randhound.NewProof(suite, g, h, core)

	f, err := q.Verify(xG, xH)
	if err != nil {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", err, f)
	}

}

func TestPVSS(t *testing.T) {

	suite := edwards.NewAES128SHA256Ed25519(false)

	base := []byte("This is a PVSS test.")
	h, _ := suite.Point().Pick(base, suite.Cipher([]byte("H")))

	threshold := 3
	X1, _ := suite.Point().Pick([]byte("X1"), random.Stream)
	X2, _ := suite.Point().Pick([]byte("X2"), random.Stream)
	X3, _ := suite.Point().Pick([]byte("X3"), random.Stream)
	X4, _ := suite.Point().Pick([]byte("X4"), random.Stream)
	X5, _ := suite.Point().Pick([]byte("X5"), random.Stream)
	X := []abstract.Point{X1, X2, X3, X4, X5}

	pvss := randhound.NewPVSS(suite, h, threshold)

	sX, core, pb, _ := pvss.Split(X)

	index := []int{1, 2, 3, 4, 5}
	pbx := [][]byte{pb, pb, pb, pb, pb}
	sH, err := pvss.Reconstruct(pbx, index)
	if err != nil {
		t.Fatal(err)
	}

	f, err := pvss.Verify(X, sH, sX, core)
	if err != nil {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", err, f)
	}

}
