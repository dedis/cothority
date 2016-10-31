package randhound

import (
	"errors"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
)

// Package proof provides functionality to create and verify non-interactive
// zero-knowledge (NIZK) proofs for the equality of discrete logarithms (dlog).

// Proof resembles a NIZK dlog-equality proof. Allows to handle multiple proofs.
type Proof struct {
	suite abstract.Suite
	Base  []ProofBase
	Core  []ProofCore
}

// ProofBase contains the base points against which the core proof is created.
type ProofBase struct {
	g abstract.Point
	h abstract.Point
}

// ProofCore contains the core elements of the NIZK dlog-equality proof.
type ProofCore struct {
	C  abstract.Scalar // challenge
	R  abstract.Scalar // response
	VG abstract.Point  // public commitment with respect to base point G
	VH abstract.Point  // public commitment with respect to base point H
}

// NewProof creates a new NIZK dlog-equality proof.
func NewProof(suite abstract.Suite, g []abstract.Point, h []abstract.Point, core []ProofCore) (*Proof, error) {

	if len(g) != len(h) {
		return nil, errors.New("Received non-matching number of points")
	}

	n := len(g)
	base := make([]ProofBase, n)
	for i := range base {
		base[i] = ProofBase{g: g[i], h: h[i]}
	}

	return &Proof{suite: suite, Base: base, Core: core}, nil
}

// Setup initializes the proof by randomly selecting a commitment v,
// determining the challenge c = H(xG,xH,vG,vH) and the response r = v - cx.
func (p *Proof) Setup(scalar ...abstract.Scalar) ([]abstract.Point, []abstract.Point, error) {

	if len(scalar) != len(p.Base) {
		return nil, nil, errors.New("Received unexpected number of scalars")
	}

	n := len(scalar)
	p.Core = make([]ProofCore, n)
	xG := make([]abstract.Point, n)
	xH := make([]abstract.Point, n)
	for i, x := range scalar {

		xG[i] = p.suite.Point().Mul(p.Base[i].g, x)
		xH[i] = p.suite.Point().Mul(p.Base[i].h, x)

		// Commitment
		v := p.suite.Scalar().Pick(random.Stream)
		vG := p.suite.Point().Mul(p.Base[i].g, v)
		vH := p.suite.Point().Mul(p.Base[i].h, v)

		// Challenge
		cb, err := crypto.HashArgsSuite(p.suite, xG[i], xH[i], vG, vH)
		if err != nil {
			return nil, nil, err
		}
		c := p.suite.Scalar().Pick(p.suite.Cipher(cb))

		// Response
		r := p.suite.Scalar()
		r.Mul(x, c).Sub(v, r)

		p.Core[i] = ProofCore{c, r, vG, vH}
	}

	return xG, xH, nil
}

// SetupCollective is similar to Setup with the difference that the challenge
// is computed as the hash over all base points and commitments.
func (p *Proof) SetupCollective(scalar ...abstract.Scalar) ([]abstract.Point, []abstract.Point, error) {

	if len(scalar) != len(p.Base) {
		return nil, nil, errors.New("Received unexpected number of scalars")
	}

	n := len(scalar)
	p.Core = make([]ProofCore, n)
	v := make([]abstract.Scalar, n)
	xG := make([]abstract.Point, n)
	xH := make([]abstract.Point, n)
	vG := make([]abstract.Point, n)
	vH := make([]abstract.Point, n)
	for i, x := range scalar {

		xG[i] = p.suite.Point().Mul(p.Base[i].g, x)
		xH[i] = p.suite.Point().Mul(p.Base[i].h, x)

		// Commitments
		v[i] = p.suite.Scalar().Pick(random.Stream)
		vG[i] = p.suite.Point().Mul(p.Base[i].g, v[i])
		vH[i] = p.suite.Point().Mul(p.Base[i].h, v[i])
	}

	// Collective challenge
	cb, err := crypto.HashArgsSuite(p.suite, xG, xH, vG, vH)
	if err != nil {
		return nil, nil, err
	}
	c := p.suite.Scalar().Pick(p.suite.Cipher(cb))

	// Responses
	for i, x := range scalar {
		r := p.suite.Scalar()
		r.Mul(x, c).Sub(v[i], r)
		p.Core[i] = ProofCore{c, r, vG[i], vH[i]}
	}

	return xG, xH, nil
}

// Verify validates the proof(s) against the given input by checking that vG ==
// rG + c(xG) and vH == rH + c(xH) and returns the indices of those proofs that
// are valid (good) and non-valid (bad), respectively.
func (p *Proof) Verify(xG []abstract.Point, xH []abstract.Point) ([]int, []int, error) {

	if len(xG) != len(xH) {
		return nil, nil, errors.New("Received unexpected number of points")
	}

	var good, bad []int
	for i := range p.Base {
		if xG[i].Equal(p.suite.Point().Null()) || xH[i].Equal(p.suite.Point().Null()) {
			bad = append(bad, i)
		} else {
			rG := p.suite.Point().Mul(p.Base[i].g, p.Core[i].R)
			rH := p.suite.Point().Mul(p.Base[i].h, p.Core[i].R)
			cxG := p.suite.Point().Mul(xG[i], p.Core[i].C)
			cxH := p.suite.Point().Mul(xH[i], p.Core[i].C)
			a := p.suite.Point().Add(rG, cxG)
			b := p.suite.Point().Add(rH, cxH)

			if p.Core[i].VG.Equal(a) && p.Core[i].VH.Equal(b) {
				good = append(good, i)
			} else {
				bad = append(bad, i)
			}
		}
	}

	return good, bad, nil
}

// PVSS implements public verifiable secret sharing.
type PVSS struct {
	suite abstract.Suite // Suite
	h     abstract.Point // Base point for polynomial commits
	t     int            // Secret sharing threshold
}

// NewPVSS creates a new PVSS struct using the given suite, base point, and
// secret sharing threshold.
func NewPVSS(s abstract.Suite, h abstract.Point, t int) *PVSS {
	return &PVSS{suite: s, h: h, t: t}
}

// Split creates PVSS shares encrypted by the public keys in X and
// provides a NIZK encryption consistency proof for each share.
func (pv *PVSS) Split(X []abstract.Point, secret abstract.Scalar) ([]int, []abstract.Point, []ProofCore, []byte, error) {

	n := len(X)

	// Create secret sharing polynomial
	priPoly := new(poly.PriPoly).Pick(pv.suite, pv.t, secret, random.Stream)

	// Create secret set of shares
	shares := new(poly.PriShares).Split(priPoly, n)

	// Create public polynomial commitments with respect to basis H
	pubPoly := new(poly.PubPoly).Commit(priPoly, pv.h)

	// Prepare data for encryption consistency proofs ...
	share := make([]abstract.Scalar, n)
	H := make([]abstract.Point, n)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
		share[i] = shares.Share(i)
		H[i] = pv.h
	}

	// ... and create them
	proof, err := NewProof(pv.suite, H, X, nil)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	_, sX, err := proof.SetupCollective(share...)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	polyBin, err := pubPoly.MarshalBinary()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return idx, sX, proof.Core, polyBin, nil
}

// Verify checks that log_H(sH) == log_X(sX) using the given proof(s) and
// returns the indices of those proofs that are valid (good) and non-valid
// (bad), respectively.
func (pv *PVSS) Verify(H abstract.Point, X []abstract.Point, sH []abstract.Point, sX []abstract.Point, core []ProofCore) (good, bad []int, err error) {

	n := len(X)
	Y := make([]abstract.Point, n)
	for i := 0; i < n; i++ {
		Y[i] = H
	}
	proof, err := NewProof(pv.suite, Y, X, core)
	if err != nil {
		return nil, nil, err
	}
	return proof.Verify(sH, sX)
}

// Commits reconstructs a list of commits from the given polynomials and indices.
func (pv *PVSS) Commits(polyBin [][]byte, index []int) ([]abstract.Point, error) {

	if len(polyBin) != len(index) {
		return nil, errors.New("Inputs have different lengths")
	}

	n := len(polyBin)
	sH := make([]abstract.Point, n)
	for i := range sH {
		P := new(poly.PubPoly)
		P.Init(pv.suite, pv.t, pv.h)
		if err := P.UnmarshalBinary(polyBin[i]); err != nil {
			return nil, err
		}
		sH[i] = P.Eval(index[i])
	}
	return sH, nil
}

// Reveal decrypts the shares in xS using the secret key x and creates an NIZK
// decryption consistency proof for each share.
func (pv *PVSS) Reveal(x abstract.Scalar, xS []abstract.Point) ([]abstract.Point, []ProofCore, error) {

	// Decrypt shares
	S := make([]abstract.Point, len(xS))
	G := make([]abstract.Point, len(xS))
	y := make([]abstract.Scalar, len(xS))
	for i := range xS {
		S[i] = pv.suite.Point().Mul(xS[i], pv.suite.Scalar().Inv(x))
		G[i] = pv.suite.Point().Base()
		y[i] = x
	}

	proof, err := NewProof(pv.suite, G, S, nil)
	if err != nil {
		return nil, nil, err
	}
	if _, _, err := proof.Setup(y...); err != nil {
		return nil, nil, err
	}
	return S, proof.Core, nil
}

// Recover recreates the PVSS secret from the given shares.
func (pv *PVSS) Recover(pos []int, S []abstract.Point, n int) (abstract.Point, error) {

	if len(S) < pv.t {
		return nil, errors.New("Not enough shares to recover secret")
	}

	//log.Lvlf1("%v %v %v %v", pos, pv.t, len(pos), len(S))

	pp := new(poly.PubPoly).InitNull(pv.suite, pv.t, pv.suite.Point().Base())
	ps := new(poly.PubShares).Split(pp, n) // XXX: ackward way to init shares

	for i, s := range S {
		ps.SetShare(pos[i], s)
	}

	return ps.SecretCommit(), nil
}
