// Package merkle contains a n-ary merkletree implementation
package merkle

import (
	"encoding"

	"github.com/dedis/crypto/abstract"
)

// Merkle contains all information needed to hash the data into a commit-tree
type Merkle struct {
	suite abstract.Suite
}

// Hash is necessary to implement the BinaryMarshaler interface on []byte
type Hash []byte

// NewMerkle initializes
func NewMerkle(s abstract.Suite) *Merkle {
	return &Merkle{suite: s}
}

// HashCommits hashes an arbitrary number of slices of bytes (e.g. hashes of the
// children) and an arbitrary number (binary) Marshalers  (like abstract.Points)
// example H(h_child1, h_child2, V1, V2)
func (m *Merkle) HashCommits(data ...encoding.BinaryMarshaler) (Hash, error) {
	hash := m.suite.Hash()
	defer hash.Reset()
	for _, p := range data {
		b, err := p.MarshalBinary()
		if err != nil {
			return nil, err
		}
		if n, err := hash.Write(b); n != len(b) || err != nil {
			return nil, err
		}
	}
	res := make([]byte, hash.Size())
	return hash.Sum(res), nil
}

// MarshalBinary is a convenience function which makes it possible to not
// differentiate between data which should be hashed or already computed hashes
// which should be hashed (like in H(h_child1, h_child2, V1, V2))
func (h Hash) MarshalBinary() (data []byte, err error) {
	return h, nil
}
