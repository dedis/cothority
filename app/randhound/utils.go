package randhound

import (
	"sort"

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

func (p *ProtocolRandHound) chooseInsurers(Rc, Rs []byte, ignore int) ([]int, []abstract.Point) {

	// Seed PRNG for insurers selection
	var seed []byte
	seed = append(seed, Rc...)
	seed = append(seed, Rs...)
	prng := p.Host.Suite().Cipher(seed)
	_ = prng

	// Determine number of peers
	e, _ := p.Host.GetEntityList(p.Token.EntityListID)
	el := e.List

	// Choose insurers uniquely
	set := make(map[int]bool)
	var keys []int
	for len(set) < p.N {
		i := int(random.Uint64(prng) % uint64(len(p.EID)))
		// Avoid choosing ourselves as insurer and add insurer only if not done so before
		if i != ignore {
			if _, ok := set[i]; !ok {
				set[i] = true
				keys = append(keys, i)
			}
		}
	}
	sort.Ints(keys) // store the list of insurers in an ascending manner
	//dbg.Lvl1(keys)
	insurers := make([]abstract.Point, p.N)
	for i, k := range keys {
		insurers[i] = el[k+1].Public //TODO: +1 since we did -1 during the EID setup! not very nice ...
	}
	return keys, insurers
}
