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
func (p *Proof) Setup(scalar ...abstract.Scalar) ([]abstract.Point, []ProofCore, error) {

	if len(scalar) != len(p.base) {
		return nil, nil, errors.New("Received unexpected number of scalars")
	}

	n := len(scalar)
	p.core = make([]ProofCore, n)
	xG := make([]abstract.Point, n)
	xH := make([]abstract.Point, n)
	for i, x := range scalar {

		xG[i] = p.suite.Point().Mul(p.base[i].g, x)
		xH[i] = p.suite.Point().Mul(p.base[i].h, x)

		// Commitment
		v := p.suite.Scalar().Pick(random.Stream)
		vG := p.suite.Point().Mul(p.base[i].g, v)
		vH := p.suite.Point().Mul(p.base[i].h, v)

		// Challenge
		cb, err := crypto.HashArgsSuite(p.suite, xG[i], xH[i], vG, vH)
		if err != nil {
			return nil, nil, err
		}
		c := p.suite.Scalar().Pick(p.suite.Cipher(cb))

		// Response
		r := p.suite.Scalar()
		r.Mul(x, c).Sub(v, r)

		p.core[i] = ProofCore{c, r, vG, vH}
	}

	xGxH := append(xG, xH...)

	return xGxH, p.core, nil
}

// SetupCollective is similar to Setup with the difference that the challenge
// is computed as the hash over all base points and commitments.
func (p *Proof) SetupCollective(scalar ...abstract.Scalar) ([]abstract.Point, []ProofCore, error) {

	if len(scalar) != len(p.base) {
		return nil, nil, errors.New("Received unexpected number of scalars")
	}

	n := len(scalar)
	p.core = make([]ProofCore, n)
	v := make([]abstract.Scalar, n)
	xG := make([]abstract.Point, n)
	xH := make([]abstract.Point, n)
	vG := make([]abstract.Point, n)
	vH := make([]abstract.Point, n)
	for i, x := range scalar {

		xG[i] = p.suite.Point().Mul(p.base[i].g, x)
		xH[i] = p.suite.Point().Mul(p.base[i].h, x)

		// Commitments
		v[i] = p.suite.Scalar().Pick(random.Stream)    // v
		vG[i] = p.suite.Point().Mul(p.base[i].g, v[i]) // vG
		vH[i] = p.suite.Point().Mul(p.base[i].h, v[i]) // vH
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
		p.core[i] = ProofCore{c, r, vG[i], vH[i]}
	}

	xGxH := append(xG, xH...)

	return xGxH, p.core, nil
}

func (p *Proof) SetCore(core []ProofCore) {
	p.core = core
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
	H     abstract.Point
	p     *poly.PriPoly
	P     *poly.PubPoly
	s     *poly.PriShares
	S     *poly.PubShares
	proof *Proof
}

func NewPVSS(h abstract.Point) *PVSS {
	return &PVSS{H: h}
}

func (pv *PVSS) Split() {

}

func (pv *PVSS) Reveal() {

}

// Init ... XXX: rename to 'encryption proof' or so
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
	s := make([]abstract.Scalar, len(key))
	H := make([]abstract.Point, len(key))
	for i := range s {
		s[i] = pv.s.Share(i + 1)
		H[i] = h
	}
	HK := append(H, key...)

	p, err := NewProof(suite, HK...)
	if err != nil {
		return err
	}
	sHsK, core, err := p.SetupCollective(s...)
	if err != nil {
		return err
	}

	q, err := NewProof(suite, HK...)
	if err != nil {
		return err
	}
	q.SetCore(core)

	failed, err := q.Verify(sHsK...)
	if err != nil {
		return errors.New(fmt.Sprintf("Verification of proofs failed: %v\n", failed))
	}

	return nil

}
