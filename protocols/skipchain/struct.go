package skipchain

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// MessageCreate is used to notify the setup
type MessageCreate struct {
}

type StructCreate struct {
	*sda.TreeNode
	MessageCreate
}

// MessageReply returns the count of all children
type MessageReply struct {
}

type StructReply struct {
	*sda.TreeNode
	MessageReply
}

// MessagePropagate returns the new skipchain block signed by the Cothority
type MessagePropagate struct {
	Block *SkipBlock
}

type StructPropagate struct {
	*sda.TreeNode
	MessagePropagate
}

type SkipBlock struct {
	X_0      abstract.Point
	Index    int
	BackLink []crypto.HashId
	//the signature is hashing all the above
	Signature   *cosi.Signature
	ForwardLink []ForwardStruct
}

type ForwardStruct struct {
	Hash      crypto.HashId
	Signature *cosi.Signature
}

//the hash is the X_0 the index and the backlinks not the other stuff
func (s *SkipBlock) Hash() crypto.HashId {
	return nil

}
