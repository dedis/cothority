package randhound

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func (rh *RandHound) chooseInsurers(Rc, Rs []byte) (map[int]int, []abstract.Point) {

	// Seed PRNG for insurers selection
	var seed []byte
	seed = append(seed, Rc...)
	seed = append(seed, Rs...)
	prng := rh.Node.Suite().Cipher(seed)

	// Choose insurers uniquely
	set := make(map[int]bool)
	insurers := make([]abstract.Point, rh.N)
	//keys := make([]int, rh.N) // old
	keys := make(map[int]int)
	tns := rh.Tree().ListNodes()
	j := 0
	for len(set) < rh.N {
		i := int(random.Uint64(prng) % uint64(len(tns)))
		// Add insurer only if not done so before; choosing yourself as an insurer is fine; ignore leader at index 0
		if _, ok := set[i]; !ok && !tns[i].IsRoot() {
			set[i] = true
			//keys[j] = i - 1 // old
			keys[i-1] = j
			insurers[j] = tns[i].Entity.Public
			j += 1
		}
	}
	return keys, insurers
}

func (rh *RandHound) hash(bytes ...[]byte) []byte {
	return abstract.Sum(rh.Node.Suite(), bytes...)
}

func (rh *RandHound) sendToChildren(msg interface{}) error {
	for _, c := range rh.Children() {
		err := rh.SendTo(c, msg)
		if err != nil {
			return err
		}
	}
	return nil
}
