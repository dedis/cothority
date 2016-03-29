// Collection of some utility functions used in the RandHound protocol.
package randhound

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func (rh *RandHound) chooseTrustees(Rc, Rs []byte) (map[uint32]uint32, []abstract.Point) {

	// Seed PRNG for selection of trustees
	var seed []byte
	seed = append(seed, Rc...)
	seed = append(seed, Rs...)
	prng := rh.Node.Suite().Cipher(seed)

	// Choose trustees uniquely
	shareIdx := make(map[uint32]uint32)
	trustees := make([]abstract.Point, rh.Group.K)
	tns := rh.Tree().ListNodes()
	j := uint32(0)
	for uint32(len(shareIdx)) < rh.Group.K {
		i := uint32(random.Uint64(prng) % uint64(len(tns)))
		// Add trustee only if not done so before; choosing yourself as an trustee is fine; ignore leader at index 0
		if _, ok := shareIdx[i]; !ok && !tns[i].IsRoot() {
			shareIdx[i] = j // j is the share index
			trustees[j] = tns[i].Entity.Public
			j += 1
		}
	}
	return shareIdx, trustees
}

func (rh *RandHound) hash(bytes ...[]byte) []byte {
	return abstract.Sum(rh.Node.Suite(), bytes...)
}

func (rh *RandHound) nodeIdx() uint32 {
	return uint32(rh.Node.TreeNode().EntityIdx)
}

func (rh *RandHound) sendToChildren(msg interface{}) error {
	for _, c := range rh.Children() {
		if err := rh.SendTo(c, msg); err != nil {
			return err
		}
	}
	return nil
}

func (rh *RandHound) generateTranscript() {} // TODO
func (rh *RandHound) verifyTranscript()   {} // TODO
