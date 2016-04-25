package skipchain

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

// ProtocolSkipchain Genesis
type ProtocolSkipchain struct {
	SetupDone chan bool
	SkipChain map[string]*SkipBlock
}

type SkipBlock interface {
	// Hash calculates the hash, writes it to the SkipBlock and returns
	// calculated hash.
	hash() crypto.HashID
}

// SkipBlock represents a skipblock
type SkipBlockCommon struct {
	Index uint32
	// Height of that SkipBlock
	Height uint32
	// For deterministic SkipChains at what height to stop:
	// - if negative: we will use random distribution to calculate the
	// height of each new block
	// - else: the max hieght determines the hieght of the next block
	MaximumHeight uint32
	// BackLink is a slice of hashes to previous SkipBlocks
	BackLink [][]byte
	// VerifierID is a SkipBlock-protocol verifying new SkipBlocks
	VerifierID VerifierId
	// Hash is calculated on all previous values
	Hash crypto.HashID
	// the signature on the above hash
	Signature *cosi.Signature
	// ForwardLink will be calculated once future SkipBlocks are
	// available
	ForwardLink []ForwardStruct
}

type SkipBlockData struct {
	*SkipBlockCommon
	// RosterPointer points to the SkipChain of the responsible Roster
	RosterPointer string
	// Data is any data to b-e stored in that SkipBlock
	Data []byte
}

func (sbd *SkipBlockData) hash() crypto.HashID {
	suite := network.Suite
	copy := *sbd
	copy.Signature = cosi.NewSignature(suite)
	copy.Hash = nil
	copy.ForwardLink = nil
	b, err := network.MarshalRegisteredType(&copy)
	if err != nil {
		dbg.Panic("Couldn't marshal skip-block:", err)
	}
	h, err := crypto.HashBytes(suite.Hash(), b)
	if err != nil {
		dbg.Panic("Couldn't hash skip-block:", err)
	}
	// store the generated hash:
	sbd.Hash = h
	return h
}

type SkipBlockRoster struct {
	*SkipBlockCommon
	// RosterName is the name of this SkipChain
	RosterName string
	// EntityList holds the roster-definition of that SkipBlock
	EntityList *sda.EntityList
}

func (sbr *SkipBlockRoster) hash() crypto.HashID {
	return nil
}

func NewSkipBlockCommon() *SkipBlockCommon {
	return &SkipBlockCommon{
		Signature: cosi.NewSignature(network.Suite),
	}
}

// ForwardStruct has the hash of the next block and a signauter of it
type ForwardStruct struct {
	Hash []byte
	*cosi.Signature
}
