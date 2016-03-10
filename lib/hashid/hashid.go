package hashid

import (
	"bytes"
	"errors"
)

type HashId []byte // Cryptographic hash content-IDs

// for sorting arrays of HashIds
type ByHashId []HashId

func (h ByHashId) Len() int           { return len(h) }
func (h ByHashId) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h ByHashId) Less(i, j int) bool { return bytes.Compare(h[i], h[j]) < 0 }

func (id HashId) Bit(i uint) int {
	return int(id[i>>3] >> (i & 7))
}

// Find the skip-chain level of an ID
func (id *HashId) Level() int {
	var level uint
	for id.Bit(level) == 0 {
		level++
	}
	return int(level)
}

// Context for looking up content blobs by self-certifying HashId.
// Implementations can be either local (same-node) or remote (cross-node).
type HashGet interface {

	// Lookup and return the binary blob for a given content HashId.
	// Checks and returns an error if the hash doesn't match the content;
	// the caller doesn't need to check this correspondence.
	Get(id HashId) ([]byte, error)
}

// Simple local-only, map-based implementation of HashGet interface
type HashMap map[string][]byte

func (m HashMap) Put(id HashId, data []byte) {
	m[string(id)] = data
}

func (m HashMap) Get(id HashId) ([]byte, error) {
	blob, ok := m[string(id)]
	if !ok {
		return nil, errors.New("HashId not found")
	}
	return blob, nil
}
