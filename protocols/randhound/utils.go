package randhound

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func (rh *RandHound) Hash(bytes ...[]byte) []byte {
	h := rh.Node.Suite().Hash()
	for _, b := range bytes {
		h.Write(b)
	}
	return h.Sum(nil)
}

func (rh *RandHound) chooseInsurers(Rc, Rs []byte) ([]int, []abstract.Point) {

	// Seed PRNG for insurers selection
	var seed []byte
	seed = append(seed, Rc...)
	seed = append(seed, Rs...)
	prng := rh.Node.Suite().Cipher(seed)

	// Choose insurers uniquely
	set := make(map[int]bool)
	insurers := make([]abstract.Point, rh.N)
	keys := make([]int, rh.N)
	j := 0
	tns := rh.Tree().ListNodes()
	for len(set) < rh.N {
		i := int(random.Uint64(prng) % uint64(rh.NumPeers))
		// Add insurer only if not done so before; choosing yourself as an insurer is fine; ignore leader
		if _, ok := set[i]; !ok && !tns[i].IsRoot() {
			set[i] = true
			keys[j] = i - 1
			insurers[j] = tns[i].Entity.Public
			j += 1
		}
	}
	return keys, insurers
}
