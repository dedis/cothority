package randhound_test

import (
	"errors"
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

	G := suite.Point().Base()
	_ = G

	base := []byte("This is a PVSS test.")
	H, _ := suite.Point().Pick(base, suite.Cipher([]byte("H")))

	threshold := 3
	n := 5
	x := make([]abstract.Scalar, n)
	X := make([]abstract.Point, n)
	for i := 0; i < n; i++ {
		x[i] = suite.Scalar().Pick(random.Stream)
		X[i] = suite.Point().Mul(nil, x[i])
	}

	secret := suite.Scalar().Pick(random.Stream)

	// (1) Share-Distribution (Dealer)
	pvss := randhound.NewPVSS(suite, H, threshold)
	sX, encProof, pb, _ := pvss.Split(X, secret)

	// (2) Share-Decryption (Trustee)
	pbx := [][]byte{pb, pb, pb, pb, pb}
	sH, err := pvss.Reconstruct(pbx)
	if err != nil {
		t.Fatal(err)
	}

	f, err := pvss.Verify(H, X, sH, sX, encProof)
	if err != nil {
		t.Fatal("Verification of discrete logarithm encryption proof(s) failed:", err, f)
	}

	S := make([]abstract.Point, n)
	decProof := make([]randhound.ProofCore, n)
	for i := 0; i < n; i++ {
		s, d, _ := pvss.Reveal(x[i], X[i], sX[i:i+1])
		S[i] = s[0]
		decProof[i] = d[0]
	}

	e, err := pvss.Verify(G, S, X, sX, decProof)
	if err != nil {
		t.Fatal("Verification of discrete logarithm decryption proof(s) failed:", err, e)
	}

	// (3) Share-Recovery (Dealer)
	recovered, err := pvss.Recover(S)
	if err != nil {
		t.Fatal(err)
	}

	if !(suite.Point().Mul(nil, secret).Equal(recovered)) {
		t.Fatal(errors.New("Recovered incorrect shared secret"))
	}
}
