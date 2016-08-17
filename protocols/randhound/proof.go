package randhound

import (
	"errors"
	"fmt"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
)

// Package (XXX: name) provides functionality to show equality of discrete
// logarithms (dlog) through non-interactive zero-knowledge (NIZK) proofs.

// Proof resembles a NIZK dlog-equality proof. Allows to hold multiple proofs.
type Proof struct {
	suite abstract.Suite
	base  []ProofBase
	core  []ProofCore
}

// ProofBase contains the base points against which the core proof is created.
type ProofBase struct {
	g abstract.Point
	h abstract.Point
}

// ProofCore contains the core elements of the NIZK dlog-equality proof.
type ProofCore struct {
	c  abstract.Scalar // challenge
	r  abstract.Scalar // response
	vG abstract.Point  // public commitment with respect to base point G
	vH abstract.Point  // public commitment with respect to base point H
}

// NewProof creates a new NIZK dlog-equality proof.
func NewProof(suite abstract.Suite, point ...abstract.Point) (*Proof, error) {

	if len(point)%2 != 0 {
		return nil, errors.New("Received odd number of points")
	}

	n := len(point) / 2
	base := make([]ProofBase, n)
	for i := 0; i < n; i++ {
		base[i] = ProofBase{g: point[i], h: point[n+i]}
	}

	return &Proof{suite: suite, base: base}, nil
}

// Setup initializes the proof by randomly selecting a commitment v and then
// determining the challenge c = H(xG,xH,vG,vH) and the response r = v - cx.
func (p *Proof) Setup(scalar ...abstract.Scalar) error {

	if len(scalar) != len(p.base) {
		return errors.New("Received unexpected number of scalars")
	}

	p.core = make([]ProofCore, len(scalar))
	for i, x := range scalar {

		xG := p.suite.Point().Mul(p.base[i].g, x)
		xH := p.suite.Point().Mul(p.base[i].h, x)

		// Commitment
		v := p.suite.Scalar().Pick(random.Stream)
		vG := p.suite.Point().Mul(p.base[i].g, v)
		vH := p.suite.Point().Mul(p.base[i].h, v)

		// Challenge
		cb, err := crypto.HashArgsSuite(p.suite, xG, xH, vG, vH)
		if err != nil {
			return err
		}
		c := p.suite.Scalar().Pick(p.suite.Cipher(cb))

		// Response
		r := p.suite.Scalar()
		r.Mul(x, c).Sub(v, r)

		p.core[i] = ProofCore{c, r, vG, vH}
	}

	return nil
}

// SetupCollective is similar to Setup with the difference that the challenge
// is computed as the hash over all base points and commitments.
func (p *Proof) SetupCollective(scalar ...abstract.Scalar) error {

	if len(scalar) != len(p.base) {
		return errors.New("Received number of points does not match number of base points")
	}

	n := len(scalar)
	p.core = make([]ProofCore, n)
	v := make([]abstract.Scalar, n)
	xG := make([]abstract.Point, n)
	xH := make([]abstract.Point, n)
	V := make([]abstract.Point, 2*n) // vG vH
	for i, x := range scalar {

		xG[i] = p.suite.Point().Mul(p.base[i].g, x)
		xH[i] = p.suite.Point().Mul(p.base[i].h, x)

		// Commitments
		v[i] = p.suite.Scalar().Pick(random.Stream)     // v
		V[i] = p.suite.Point().Mul(p.base[i].g, v[i])   // vG
		V[n+i] = p.suite.Point().Mul(p.base[i].h, v[i]) // vH
	}

	// Collective challenge
	cb, err := crypto.HashArgsSuite(p.suite, xG, xH, V)
	if err != nil {
		return err
	}
	c := p.suite.Scalar().Pick(p.suite.Cipher(cb))

	// Responses
	for i, x := range scalar {
		r := p.suite.Scalar()
		r.Mul(x, c).Sub(v[i], r)
		p.core[i] = ProofCore{c, r, V[i], V[n+i]}
	}

	return nil
}

// Verify validates the proof against the given input by checking that
// vG == rG + c(xG) and vH == rH + c(xH).
func (p *Proof) Verify(point ...abstract.Point) ([]int, error) {

	if len(point) != 2*len(p.base) {
		return nil, errors.New("Received unexpected number of points")
	}

	n := len(point) / 2
	var failed []int
	for i := 0; i < len(p.base); i++ {

		rG := p.suite.Point().Mul(p.base[i].g, p.core[i].r)
		rH := p.suite.Point().Mul(p.base[i].h, p.core[i].r)
		cxG := p.suite.Point().Mul(point[i], p.core[i].c)
		cxH := p.suite.Point().Mul(point[n+i], p.core[i].c)
		a := p.suite.Point().Add(rG, cxG)
		b := p.suite.Point().Add(rH, cxH)

		if !(p.core[i].vG.Equal(a) && p.core[i].vH.Equal(b)) {
			failed = append(failed, i)
		}
	}
	return failed, nil
}

// PVSS implements public verifiable secret sharing
type PVSS struct {
	p     *poly.PriPoly
	P     *poly.PubPoly
	s     *poly.PriShares
	S     *poly.PubShares
	proof *Proof
}

// Init ...
func (pv *PVSS) Setup(suite abstract.Suite, threshold int, base []byte, key []abstract.Point) error {

	// Create secret sharing polynomial
	pv.p = new(poly.PriPoly).Pick(suite, threshold, nil, random.Stream)

	// Create secret set of shares
	pv.s = new(poly.PriShares).Split(pv.p, len(key)+1)

	// Compute base point H (TODO: use Elligator to compute h?)
	h, _ := suite.Point().Pick(base, suite.Cipher([]byte("H")))

	// Create public polynomial commitments
	pv.P = new(poly.PubPoly).Commit(pv.p, h)

	// Create public share commitments
	pv.S = new(poly.PubShares).Split(pv.P, threshold)

	// Encrypt shares with keys; TODO: ensure that s[0] is not used for encrypting keys; check potential one-off index problem
	si := make([]abstract.Scalar, len(key))
	hs := make([]abstract.Point, len(key))
	for i := range si {
		si[i] = pv.s.Share(i + 1)
		hs[i] = h
	}
	hs = append(hs, key...)

	proof, err := NewProof(suite, hs...)
	if err != nil {
		return err
	}
	proof.SetupCollective(si...)

	Si := make([]abstract.Point, len(key))
	Hi := make([]abstract.Point, len(key))
	for i := range Si {
		Si[i] = suite.Point().Mul(key[i], si[i])
		Hi[i] = suite.Point().Mul(h, si[i])
	}
	Hi = append(Hi, Si...)

	failed, err := proof.Verify(Hi...)
	fmt.Println("failed:", failed)
	if err != nil {
		return err
	}

	return nil

}
