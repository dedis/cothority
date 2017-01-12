package pbft

import (
	"github.com/dedis/cothority/byzcoin/blockchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

func init() {
	for _, i := range []interface{}{
		PrePrepare{},
		Prepare{},
		Commit{},
		Finish{},
	} {
		network.RegisterMessage(i)
	}
}

// Messages which will be sent around by the most naive PBFT simulation in
// "byzcoin"

// PrePrepare message
type PrePrepare struct {
	*blockchain.TrBlock
}

type prePrepareChan struct {
	*onet.TreeNode
	PrePrepare
}

// Prepare is the prepare packet
type Prepare struct {
	HeaderHash string
}

type prepareChan struct {
	*onet.TreeNode
	Prepare
}

// Commit is the commit packet in the protocol
type Commit struct {
	HeaderHash string
}

type commitChan struct {
	*onet.TreeNode
	Commit
}

// Finish is just to tell the others node that the protocol is finished
type Finish struct {
	Done string
}

type finishChan struct {
	*onet.TreeNode
	Finish
}
