package randhound

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func (p *ProtocolRandHound) Hash(bytes ...[]byte) []byte {
	h := p.ProtocolStruct.Host.Suite().Hash()
	for _, b := range bytes {
		h.Write(b)
	}
	return h.Sum(nil)
}

func (p *ProtocolRandHound) chooseInsurers(Rc, Rs []byte) []abstract.Point {

	// Seed PRNG for insurers selection
	var seed []byte
	seed = append(seed, Rc...)
	seed = append(seed, Rs...)
	prng := p.Host.Suite().Cipher(seed)
	_ = prng

	e, _ := p.Host.GetEntityList(p.Token.EntityListID)
	el := e.List
	npeers := len(el) - 1
	_ = npeers

	// emulating a set through a map: we want to choose p.N unique public keys
	set := make(map[int]abstract.Point)
	for len(set) < p.N {
		i := int(random.Uint64(prng)%uint64(npeers)) + 1 // +1: we want to avoid the leader which has index 0
		set[i] = el[i].Public
	}
	var insurers []abstract.Point
	for _, v := range set {
		insurers = append(insurers, v)
	}
	return insurers
}
