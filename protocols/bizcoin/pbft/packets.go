package pbft

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
)

// Messages which will be sent around by the most naive PBFT simulation in
// "bizcoin"

// PrePrepare message
type PrePrepare struct {
	*blockchain.TrBlock
}

type prePrepareChan struct {
	*sda.TreeNode
	PrePrepare
}

type Prepare struct {
	HeaderHash string
}

type prepareChan struct {
	*sda.TreeNode
	Prepare
}

type Commit struct {
	HeaderHash string
}

type commitChan struct {
	*sda.TreeNode
	Commit
}

type Finish struct{}

type finishChan struct {
	*sda.TreeNode
	Finish
}
