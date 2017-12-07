// +build experimental

package randhound_test

import (
	"testing"

	"github.com/dedis/cothority/randhound"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet/log"
)

var tSuite = ed25519.NewBlakeSHA256Ed25519()

func TestProof(t *testing.T) {
	// 1st set of base points
	g1, _ := tSuite.Point().Pick([]byte("G1"), random.Stream)
	h1, _ := tSuite.Point().Pick([]byte("H1"), random.Stream)

	// 1st secret value
	x := tSuite.Scalar().Pick(random.Stream)

	// 2nd set of base points
	g2, _ := tSuite.Point().Pick([]byte("G2"), random.Stream)
	h2, _ := tSuite.Point().Pick([]byte("H2"), random.Stream)

	// 2nd secret value
	y := tSuite.Scalar().Pick(random.Stream)

	// Create proofs
	g := []kyber.Point{g1, g2}
	h := []kyber.Point{h1, h2}
	p, err := randhound.NewProof(suite, g, h, nil)
	log.ErrFatal(err)

	xG, xH, err := p.Setup(x, y)
	log.ErrFatal(err)

	// Verify proofs
	q, err := randhound.NewProof(suite, g, h, p.Core)
	log.ErrFatal(err)

	_, bad, err := q.Verify(xG, xH)
	log.ErrFatal(err)

	if len(bad) != 0 {
		log.Fatalf("Some proofs failed: %v", bad)
	}
}

func TestProofCollective(t *testing.T) {
	// 1st set of base points
	g1, _ := tSuite.Point().Pick([]byte("G1"), random.Stream)
	h1, _ := tSuite.Point().Pick([]byte("H1"), random.Stream)

	// 1st secret value
	x := tSuite.Scalar().Pick(random.Stream)

	// 2nd set of base points
	g2, _ := tSuite.Point().Pick([]byte("G2"), random.Stream)
	h2, _ := tSuite.Point().Pick([]byte("H2"), random.Stream)

	// 2nd secret value
	y := tSuite.Scalar().Pick(random.Stream)

	// Create proof
	g := []kyber.Point{g1, g2}
	h := []kyber.Point{h1, h2}
	p, err := randhound.NewProof(suite, g, h, nil)
	log.ErrFatal(err)

	xG, xH, err := p.SetupCollective(x, y)
	log.ErrFatal(err)

	// Verify proof
	q, err := randhound.NewProof(suite, g, h, p.Core)
	log.ErrFatal(err)

	_, bad, err := q.Verify(xG, xH)
	log.ErrFatal(err)

	if len(bad) != 0 {
		log.Fatalf("Some proofs failed: %v", bad)
	}

}

func TestPVSS(t *testing.T) {
	G := tSuite.Point().Base()
	H, _ := tSuite.Point().Pick(nil, tSuite.Cipher([]byte("H")))

	n := 10
	threshold := 2*n/3 + 1
	x := make([]kyber.Scalar, n) // trustee private keys
	X := make([]kyber.Point, n)  // trustee public keys
	index := make([]int, n)
	for i := 0; i < n; i++ {
		x[i] = tSuite.Scalar().Pick(random.Stream)
		X[i] = tSuite.Point().Mul(nil, x[i])
		index[i] = i
	}

	// Scalar of shared secret
	secret := tSuite.Scalar().Pick(random.Stream)

	// (1) Share-Distribution (Dealer)
	pvss := randhound.NewPVSS(suite, H, threshold)
	idx, sX, encProof, pb, err := pvss.Split(X, secret)
	log.ErrFatal(err)

	// (2) Share-Decryption (Trustee)
	pbx := make([][]byte, n)
	for i := 0; i < n; i++ {
		pbx[i] = pb // NOTE: polynomials can be different
	}
	sH, err := pvss.Commits(pbx, index)
	log.ErrFatal(err)

	// Check that log_H(sH) == log_X(sX) using encProof
	_, bad, err := pvss.Verify(H, X, sH, sX, encProof)
	log.ErrFatal(err)

	if len(bad) != 0 {
		log.Fatalf("Some proofs failed: %v", bad)
	}

	// Decrypt shares
	S := make([]kyber.Point, n)
	decProof := make([]randhound.ProofCore, n)
	for i := 0; i < n; i++ {
		s, d, err := pvss.Reveal(x[i], sX[i:i+1])
		log.ErrFatal(err)
		S[i] = s[0]
		decProof[i] = d[0]
	}

	// Check that log_G(S) == log_X(sX) using decProof
	_, bad, err = pvss.Verify(G, S, X, sX, decProof)
	log.ErrFatal(err)

	if len(bad) != 0 {
		log.Fatalf("Some proofs failed: %v", bad)
	}

	// (3) Secret-Recovery (Dealer)
	recovered, err := pvss.Recover(idx, S, len(S))
	log.ErrFatal(err)

	// Verify recovered secret
	if !(tSuite.Point().Mul(nil, secret).Equal(recovered)) {
		log.Fatalf("Recovered incorrect shared secret")
	}
}
