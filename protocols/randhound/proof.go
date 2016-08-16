package randhound

import (
	"errors"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/crypto/abstract"
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
	gv abstract.Point  // public commitment with respect to base point G
	hv abstract.Point  // public commitment with respect to base point H
}

// NewProof creates a new NIZK dlog-equality proof.
func NewProof(suite abstract.Suite, point ...abstract.Point) (*Proof, error) {

	if len(point)%2 != 0 {
		return nil, errors.New("Received odd number of points")
	}

	base := make([]ProofBase, len(point)/2)
	for i := 0; i < len(point)/2; i++ {
		base[i] = ProofBase{g: point[2*i], h: point[2*i+1]}
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

		gx := p.suite.Point().Mul(p.base[i].g, x)
		hx := p.suite.Point().Mul(p.base[i].h, x)

		// Commitment
		v := p.suite.Scalar().Pick(random.Stream)
		gv := p.suite.Point().Mul(p.base[i].g, v)
		hv := p.suite.Point().Mul(p.base[i].h, v)

		// Challenge
		cb, err := crypto.HashArgsSuite(p.suite, gx, hx, gv, hv)
		if err != nil {
			return err
		}
		c := p.suite.Scalar().Pick(p.suite.Cipher(cb))

		// Response
		r := p.suite.Scalar()
		r.Mul(x, c).Sub(v, r)

		p.core[i] = ProofCore{c, r, gv, hv}
	}

	return nil
}

// SetupCollective is similar to Setup with the difference that the challenge
// is computed as the hash over all base points and commitments.
func (p *Proof) SetupCollective(scalar ...abstract.Scalar) error {

	if len(scalar) != len(p.base) {
		return errors.New("Received number of points does not match number of base points")
	}

	p.core = make([]ProofCore, len(scalar))
	v := make([]abstract.Scalar, len(scalar))
	X := make([]abstract.Point, len(scalar))
	Y := make([]abstract.Point, len(scalar))
	V := make([]abstract.Point, 2*len(scalar))
	for i, x := range scalar {

		X[i] = p.suite.Point().Mul(p.base[i].g, x) // xG
		Y[i] = p.suite.Point().Mul(p.base[i].h, x) // xH

		// Commitments
		v[i] = p.suite.Scalar().Pick(random.Stream)       // v
		V[2*i] = p.suite.Point().Mul(p.base[i].g, v[i])   // vG
		V[2*i+1] = p.suite.Point().Mul(p.base[i].h, v[i]) // vH
	}

	// Collective challenge
	cb, err := crypto.HashArgsSuite(p.suite, X, Y, V)
	if err != nil {
		return err
	}
	c := p.suite.Scalar().Pick(p.suite.Cipher(cb))

	// Responses
	for i, x := range scalar {
		r := p.suite.Scalar()
		r.Mul(x, c).Sub(v[i], r)
		p.core[i] = ProofCore{c, r, V[2*i], V[2*i+1]}
	}

	return nil
}

// Verify validates the proof against the given input by checking that
// vG == rG + c(xG) and vH == rH + c(xH).
func (p *Proof) Verify(point ...abstract.Point) ([]int, error) {

	if len(point) != 2*len(p.base) {
		return nil, errors.New("Received unexpected number of points")
	}

	var failed []int
	for i := 0; i < len(p.base); i++ {

		gr := p.suite.Point().Mul(p.base[i].g, p.core[i].r)
		hr := p.suite.Point().Mul(p.base[i].h, p.core[i].r)
		gxc := p.suite.Point().Mul(point[2*i], p.core[i].c)
		hxc := p.suite.Point().Mul(point[2*i+1], p.core[i].c)
		a := p.suite.Point().Add(gr, gxc)
		b := p.suite.Point().Add(hr, hxc)

		if !(p.core[i].gv.Equal(a) && p.core[i].hv.Equal(b)) {
			failed = append(failed, i)
		}
	}
	return failed, nil
}
