package randhound

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func (rh *RandHound) Hash(bytes ...[]byte) []byte {
	h := rh.ProtocolStruct.Host.Suite().Hash()
	for _, b := range bytes {
		h.Write(b)
	}
	return h.Sum(nil)
}

func (rh *RandHound) chooseInsurers(Rc, Rs []byte, ignore int) ([]int, []abstract.Point) {

	// Seed PRNG for insurers selection
	var seed []byte
	seed = append(seed, Rc...)
	seed = append(seed, Rs...)
	prng := rh.Host.Suite().Cipher(seed)
	_ = prng

	// Determine number of peers
	e, _ := rh.Host.GetEntityList(rh.Token.EntityListID)
	el := e.List

	// Choose insurers uniquely
	set := make(map[int]bool)
	insurers := make([]abstract.Point, rh.N)
	keys := make([]int, rh.N)
	j := 0
	for len(set) < rh.N {
		i := int(random.Uint64(prng) % uint64(len(rh.EID)))
		// Avoid choosing the 'ignore' index as insurer and add insurer only if not done so before
		if i != ignore {
			if _, ok := set[i]; !ok {
				set[i] = true
				keys[j] = i
				insurers[j] = el[i+1].Public //NOTE: i+1 since we did -1 during the EID setup! TODO: not very nice ...
				j += 1
			}
		}
	}
	return keys, insurers
}
