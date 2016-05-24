package pbft

import (
	"gopkg.in/dedis/cothority.v0/lib/sda"
	"gopkg.in/dedis/cothority.v0/protocols/byzcoin/blockchain"
)

// Messages which will be sent around by the most naive PBFT simulation in
// "byzcoin"

// PrePrepare message
type PrePrepare struct {
	*blockchain.TrBlock
}

type prePrepareChan struct {
	*sda.TreeNode
	PrePrepare
}

// Prepare is the prepare packet
type Prepare struct {
	HeaderHash string
}

type prepareChan struct {
	*sda.TreeNode
	Prepare
}

// Commit is the commit packet in the protocol
type Commit struct {
	HeaderHash string
}

type commitChan struct {
	*sda.TreeNode
	Commit
}

// Finish is just to tell the others node that the protocol is finished
type Finish struct {
	Done string
}

type finishChan struct {
	*sda.TreeNode
	Finish
}
