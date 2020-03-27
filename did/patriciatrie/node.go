package patriciatrie

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

type node interface {
	String() string
	hash() node
}

type shortNodeFlag byte

const (
	EVEN_EXT shortNodeFlag = iota
	ODD_EXT
	EVEN_LEAF
	ODD_LEAF
)

type (
	// branchNode represents a 17 element node with the first 16 elements referring
	// to another node and the 17th element referring to a value
	branchNode [17]node
	// shortNode presents an extension node or a leaf node
	shortNode struct {
		// remainingPath stores the *compacted decoded* remaining path
		remainingPath []byte
		// value refers to a user given value in case of a leaf node or another node
		// in case of an extension node
		value node
		// flag stores metadata about the shortNode that helps distinguish between
		// an extension or a leaf node and the length of compact decoded remaining path
		// (even/odd)
		flag shortNodeFlag
	}
	// hashnode stores the sha3-256 hash of an rlp encoded node
	hashnode []byte
	// valuenode stores the user supplied value
	valuenode []byte
)

func (v valuenode) String() string {
	return fmt.Sprintf("%x", []byte(v))
}

func (v valuenode) hash() node {
	return v
}

func (h hashnode) String() string {
	return fmt.Sprintf("%x", []byte(h))
}

func (h hashnode) hash() node {
	return h
}

func (b branchNode) String() string {
	return fmt.Sprintf("%s: %s", b[:16], b[16])
}

// EncodeRLP recursively encodes the branch node using
// RLP encoding
func (b branchNode) EncodeRLP(w io.Writer) error {
	var data []interface{}
	for i := range b {
		if b[i] == nil {
			data = append(data, []byte{})
		} else {
			data = append(data, b[i].hash())
		}
	}
	return rlp.Encode(w, data)
}

// hash returns the sha3-256 hash of a node if its RLP
// encoding >= 32 bytes. Otherwise, it returns the node
// itself
func (b branchNode) hash() node {
	buf, _ := rlp.EncodeToBytes(b)
	if len(buf) < 32 {
		return b
	}
	hash := hashData(buf)
	return hashnode(hash)
}

func hashData(data []byte) []byte {
	sha256 := sha3.New256()
	sha256.Write(data)
	return sha256.Sum(nil)
}

func (s shortNode) String() string {
	return fmt.Sprintf("(%d) <%x>: %s", s.flag, s.remainingPath, s.value)
}

// EncodeRLP recursively encodes the RLP encoding of the shortNode
func (s shortNode) EncodeRLP(w io.Writer) error {
	compactPath := encodePath(s.remainingPath, s.flag)
	toEncode := []interface{}{compactPath, s.value.hash()}
	return rlp.Encode(w, toEncode)
}

func (s shortNode) isTerm() byte {
	if s.flag < 2 {
		return 0
	}
	return 2
}

// hash returns the sha3-256 hash of a node if its RLP
// encoding >= 32 bytes. Otherwise, it returns the node
// itself
func (s shortNode) hash() node {
	buf, _ := rlp.EncodeToBytes(s)
	if len(buf) < 32 {
		return s
	}
	ret := hashData(buf)
	return hashnode(ret)
}

// encodePath returns the compact encoding of `remainingPath`
// using the specification in
// https://github.com/ethereum/wiki/wiki/Patricia-Tree#specification-compact-encoding-of-hex-sequence-with-optional-terminator
func encodePath(remainingPath []byte, flag shortNodeFlag) []byte {
	data := []byte{byte(flag)}
	if flag == EVEN_LEAF || flag == EVEN_EXT {
		data = append(data, 0)
	}
	data = append(data, remainingPath...)

	result := []byte{}
	for i := 0; i < len(data); i += 2 {
		result = append(result, (data[i]<<4)|(data[i+1]))
	}
	return result
}

// hexToNibbles is a helper function that takes in a slice of bytes
// and returns a slice of nibbles
func hexToNibbles(hex []byte) []byte {
	nibbles := []byte{}
	for _, b := range hex {
		nibbles = append(nibbles, b>>4, b&0x0f)
	}
	return nibbles
}
