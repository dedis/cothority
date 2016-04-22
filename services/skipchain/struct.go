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
	*cosi.Signature
	ForwardLink []ForwardStruct
	//transmitted for the signature - not included in the hash
	*sda.EntityList
}

func NewSkipBlock(el *sda.EntityList) *SkipBlock {
	X0 := network.Suite.Point().Null()
	nodes := el.List
	for _, tn := range nodes {
		X0.Add(X0, tn.Public)
	}
	return &SkipBlock{
		X0:         X0,
		EntityList: el,
		Signature:  cosi.NewSignature(network.Suite),
		Index:      0,
	}
}

// ForwardStruct has the hash of the next block and a signauter of it
type ForwardStruct struct {
	Hash []byte
	*cosi.Signature
}
