package randhound

import (
	"bytes"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

// Package (XXX: name) provides functionality to show equality of discrete
// logarithms (dlog) through non-interactive zero-knowledge (NIZK) proofs.

// Proof resembles a NIZK dlog-equality proof.
type Proof struct {
	suite abstract.Suite  // suite
	g     abstract.Point  // first base point G
	h     abstract.Point  // second base point H
	c     abstract.Scalar // challenge
	r     abstract.Scalar // response
	gv    abstract.Point  // public commitment with respect to G
	hv    abstract.Point  // public commitment with respect to H
}

// NewProof creates a new NIZK dlog-equality proof.
func NewProof(suite abstract.Suite, g abstract.Point, h abstract.Point) *Proof {
	return &Proof{suite: suite, g: g, h: h}
}

// Setup initializes the proof by randomly selecting a commitment v and then
// determining the challenge c = H(G,H,x,v) and the response r = v - cx.
func (p *Proof) Setup(x abstract.Scalar) error {

	// Commitment
	v := p.suite.Scalar().Pick(random.Stream)
	p.gv = p.suite.Point().Mul(p.g, v)
	p.hv = p.suite.Point().Mul(p.h, v)

	// Challenge
	buf := new(bytes.Buffer)

	gb, err := p.g.MarshalBinary()
	if err != nil {
		return err
	}
	buf.Write(gb)

	hb, err := p.h.MarshalBinary()
	if err != nil {
		return err
	}
	buf.Write(hb)

	xb, err := x.MarshalBinary()
	if err != nil {
		return err
	}
	buf.Write(xb)

	vb, err := v.MarshalBinary()
	if err != nil {
		return err
	}
	buf.Write(vb)

	cb := abstract.Sum(p.suite, buf.Bytes())
	c := p.suite.Scalar().Pick(p.suite.Cipher(cb))
	p.c = c

	// Response
	r := p.suite.Scalar()
	r.Mul(x, c).Sub(v, r)
	p.r = r

	return nil
}

// Verify validates the proof against the given input by checking that
// v * G == r * G + c * (x * G) and v * H == r * H + c * (x * H).
func (p *Proof) Verify(gx abstract.Point, hx abstract.Point) bool {

	gr := p.suite.Point().Mul(p.g, p.r)
	hr := p.suite.Point().Mul(p.h, p.r)
	gxc := p.suite.Point().Mul(gx, p.c)
	hxc := p.suite.Point().Mul(hx, p.c)

	x := p.suite.Point().Add(gr, gxc)
	y := p.suite.Point().Add(hr, hxc)

	return p.gv.Equal(x) && p.hv.Equal(y)
}
