package skipchain

import (
	"encoding/binary"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// MessageGenesis is used for the first round
type MessageGenesis struct {
	Block *SkipBlock
}
// StructGenesis is used for the genesis round
type StructGenesis struct {
	*sda.TreeNode
	MessageGenesis
}

// MessagePropagate returns the new skipchain block signed by the Cothority
type MessagePropagate struct {
	Block *SkipBlock
}
// StructPropagate is used to send the newly signed block
type StructPropagate struct {
	*sda.TreeNode
	MessagePropagate
}
// SkipBlock represents a skipblock
type SkipBlock struct {
	X0      abstract.Point
	Index    uint32
	BackLink [][]byte
	//the signature is hashing all the above
	Signature   *cosi.Signature
	ForwardLink []ForwardStruct
	Nodes       []*sda.TreeNode //transmited for the signature assigned to null before storage
}
// ForwardStruct has the hash of the next block and a signauter of it
type ForwardStruct struct {
	Hash      []byte
	Signature *cosi.Signature
}

//Hash is the X0 the index and the backlinks not the other stuff
func (s *SkipBlock) Hash() []byte {
	h := network.Suite.Hash()
	s.X0.MarshalTo(h)
	err := binary.Write(h, binary.LittleEndian, s.Index)
	if err != nil {

		dbg.Fatal(err)
	}
	for _, b := range s.BackLink {
		h.Write(b)
	}
	return h.Sum(nil)
}
