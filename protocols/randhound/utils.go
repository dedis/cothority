// Collection of some utility functions used in the RandHound protocol.
package randhound

import (
	"bytes"
	"encoding/binary"
	"time"

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

func (rh *RandHound) newGroup(nodes int, trustees int) (*Group, []byte, error) {

	n := uint32(nodes)    // Number of nodes (peers + leader)
	k := uint32(trustees) // Number of trustees (= shares generaetd per peer)
	buf := new(bytes.Buffer)

	// Setup group parameters: note that T <= R <= K must hold;
	// T = R for simplicity, might change later
	gp := [6]uint32{
		n,           // N: total number of nodes (peers + leader)
		n / 3,       // F: maximum number of Byzantine nodes tolerated
		n - (n / 3), // L: minimum number of non-Byzantine nodes required
		k,           // K: total number of trustees (= shares generated per peer)
		(k + 1) / 2, // R: minimum number of signatures needed to certify a deal
		(k + 1) / 2, // T: minimum number of shares needed to reconstruct a secret
	}

	// Include public keys of all nodes into group ID
	for _, x := range rh.Tree().ListNodes() {
		pub, err := x.Entity.Public.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		if err = binary.Write(buf, binary.LittleEndian, pub); err != nil {
			return nil, nil, err
		}
	}

	// Include group parameters into group ID
	for _, g := range gp {
		if err := binary.Write(buf, binary.LittleEndian, g); err != nil {
			return nil, nil, err
		}
	}

	return &Group{
		N: gp[0],
		F: gp[1],
		L: gp[2],
		K: gp[3],
		R: gp[4],
		T: gp[5]}, rh.hash(buf.Bytes()), nil
}

func (rh *RandHound) newSession(public abstract.Point, purpose string, time time.Time) (*Session, []byte, error) {

	pub, err := public.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	tm, err := time.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	return &Session{
		Fingerprint: pub,
		Purpose:     purpose,
		Time:        time}, rh.hash(pub, []byte(purpose), tm), nil
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
