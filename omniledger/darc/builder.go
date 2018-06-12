package darc

import "github.com/dedis/cothority/omniledger/darc/expression"

func NewDarcBuilder() darcBuilder {
	return darcBuilder{
		d:   Darc{},
		err: nil,
	}
}

type darcBuilder struct {
	d   Darc
	err error
}

func (b *darcBuilder) Build() (*Darc, error) {
	if b.err != nil {
		return nil, b.err
	}
	return &b.d, nil
}

func (b *darcBuilder) SetVersion(v uint64) {
	b.d.Version = v
}

func (b *darcBuilder) SetDescription(desc []byte) {
	b.d.Description = desc
}

func (b *darcBuilder) SetBaseID(baseID ID) {
	b.d.BaseID = baseID
}

func (b *darcBuilder) AddRule(a Action, expr expression.Expr) {
	err := b.d.Rules.AddRule(a, expr)
	if b.err == nil {
		b.err = err
	}
}

func (b *darcBuilder) UpdateEvolution(expr expression.Expr) {
	err := b.d.Rules.UpdateEvolution(expr)
	b.err = err
}

// TODO more ways of changing the rules.

func (b *darcBuilder) Sign(signers ...Signer) {
	// TODO create request, loop all signers, sign the darc
}

func (b *darcBuilder) SetVerificationDarcs(darcs ...*Darc) {
	b.d.VerificationDarcs = darcs
}

func (b *darcBuilder) AddVerificationDarc(darc *Darc) {
	b.d.VerificationDarcs = append(b.d.VerificationDarcs, darc)
}

func (d *Darc) ToBuilder() darcBuilder {
	return darcBuilder{
		d:   *d.Copy(),
		err: nil,
	}
}

func (d *Darc) StartEvolution() darcBuilder {
	b := darcBuilder{
		d:   *d.Copy(),
		err: nil,
	}
	b.d.Version++
	// TODO need to change anything else?
	return b
}
