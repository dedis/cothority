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
	q, err := randhound.NewProof(suite, g, h, core)
	if err != nil {
		t.Fatal(err)
	}

	f, err := q.Verify(xG, xH)
	if err != nil {
		t.Fatal("Verification of discrete logarithm proof(s) failed:", err, f)
	}

}

func TestPVSS(t *testing.T) {

	suite := edwards.NewAES128SHA256Ed25519(false)

	G := suite.Point().Base()
	H, _ := suite.Point().Pick(nil, suite.Cipher([]byte("H")))

	n := 10
	threshold := 2*n/3 + 1
	x := make([]abstract.Scalar, n) // trustee private keys
	X := make([]abstract.Point, n)  // trustee public keys
	index := make([]int, n)
	for i := 0; i < n; i++ {
		x[i] = suite.Scalar().Pick(random.Stream)
		X[i] = suite.Point().Mul(nil, x[i])
		index[i] = i
	}

	// Scalar of shared secret
	secret := suite.Scalar().Pick(random.Stream)

	// (1) Share-Distribution (Dealer)
	pvss := randhound.NewPVSS(suite, H, threshold)
	sX, encProof, pb, err := pvss.Split(X, secret)
	if err != nil {
		t.Fatal(err)
	}

	// (2) Share-Decryption (Trustee)
	pbx := make([][]byte, n)
	for i := 0; i < n; i++ {
		pbx[i] = pb // NOTE: polynomials can be  different
	}
	sH, err := pvss.Commits(pbx, index)
	if err != nil {
		t.Fatal(err)
	}

	// Check that log_H(sH) == log_X(sX) using encProof
	f, err := pvss.Verify(H, X, sH, sX, encProof)
	if err != nil {
		t.Fatal("encProof:", err, f)
	}

	// Decrypt shares
	S := make([]abstract.Point, n)
	decProof := make([]randhound.ProofCore, n)
	for i := 0; i < n; i++ {
		s, d, err := pvss.Reveal(x[i], sX[i:i+1])
		if err != nil {
			t.Fatal(err)
		}
		S[i] = s[0]
		decProof[i] = d[0]
	}

	// Check that log_G(S) == log_X(sX) using decProof
	e, err := pvss.Verify(G, S, X, sX, decProof)
	if err != nil {
		t.Fatal("decProof:", err, e)
	}

	// (3) Share-Recovery (Dealer)
	recovered, err := pvss.Recover(S)
	if err != nil {
		t.Fatal(err)
	}

	// Verify recovered secret
	if !(suite.Point().Mul(nil, secret).Equal(recovered)) {
		t.Fatal(errors.New("Recovered incorrect shared secret"))
	}
}
