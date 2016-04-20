package skipchain

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// ProtocolSkipchain Genesis
type ProtocolSkipchain struct {
	SetupDone chan bool
	SkipChain map[string]*SkipBlock
	LastBlock []byte
	Genesis   []byte
}

// SkipBlock represents a skipblock
type SkipBlock struct {
	X0       abstract.Point
	Index    uint32
	BackLink [][]byte
	//the signature is hashing all the above
	Signature   *cosi.Signature
	ForwardLink []ForwardStruct
	Nodes       []*sda.TreeNode //transmitted for the signature assigned to null before storage
}

func NewSkipBlock(tree *sda.Tree) *SkipBlock {
	X0 := network.Suite.Point().Null()
	nodes := tree.List()
	for _, tn := range nodes {
		X0.Add(X0, tn.Entity.Public)
	}
	return &SkipBlock{
		X0:    X0,
		Nodes: nodes,
	}
}

// ForwardStruct has the hash of the next block and a signauter of it
type ForwardStruct struct {
	Hash      []byte
	Signature *cosi.Signature
}
