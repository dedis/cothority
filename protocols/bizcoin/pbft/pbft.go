package pbft

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
)

const (
	NotFound = -1
)

// Protocol implements sda.Protocol
// we do basically the same as in http://www.pmg.lcs.mit.edu/papers/osdi99.pdf
// with the following diffs:
// there is no client/server request and reply (or first line in Figure 1)
// instead of MACs we just send around the hash of the block
// this will make the protocol faster, but the network latency will overweigh
// this skipped computation anyways
type Protocol struct {
	// the node we are represented-in
	*sda.Node
	// the suite we use
	suite abstract.Suite
	// aggregated public key of the peers
	aggregatedPublic abstract.Point
	// a flat list of all TreeNodes (similar to jvss)
	nodeList []*sda.TreeNode
	// our index in the entitylist
	index int

	// we do not care for servers or clients (just store one block here)
	trBlock *blockchain.TrBlock

	prepMsgCount   int
	commitMsgCount int
	threshold      int
	// channels:
	prePrepareChan chan prePrepareChan
	prepareChan    chan prepareChan
	commitChan     chan commitChan

	onDoneCB func()
}

func NewProtocol(n *sda.Node) (*Protocol, error) {
	pbft := new(Protocol)
	tree := n.Tree()
	pbft.Node = n
	pbft.nodeList = tree.ListNodes()
	idx := NotFound
	for i, tn := range pbft.nodeList {
		if uuid.Equal(tn.Id, n.TreeNode().Id) {
			idx = i
		}
	}
	if idx == NotFound {
		panic(fmt.Sprintf("Could not find ourselves %+v in the list of nodes %+v", n, pbft.nodeList))
	}
	pbft.index = idx
	// 2/3 * #participants == threshold FIXME the threshold is actually XXX
	pbft.threshold = int(math.Ceil(float64(len(pbft.nodeList)) * 2.0 / 3.0))
	pbft.prepMsgCount = 1
	pbft.commitMsgCount = 1

	n.RegisterChannel(&pbft.prePrepareChan)
	n.RegisterChannel(&pbft.prepareChan)
	n.RegisterChannel(&pbft.commitChan)
	return pbft, nil
}

func NewRootProtocol(n *sda.Node, trBlock *blockchain.TrBlock, onDoneCb func()) (*Protocol, error) {
	pbft, err := NewProtocol(n)
	if err != nil {
		return nil, err
	}
	pbft.trBlock = trBlock
	pbft.onDoneCB = onDoneCb
	return pbft, nil
}

// Dispatch listen on the different channels
func (p *Protocol) Dispatch() error {
	if p.IsRoot() {
		dbg.Print("Dispatch for root node")
		for {
			//var err error
			select {
			case msg := <-p.prepareChan:
				p.handlePrepare(&msg.Prepare)
			case msg := <-p.commitChan:
				p.handleCommit(&msg.Commit)
			}
		}
	} else {
		for {
			//var err error
			select {
			case msg := <-p.prePrepareChan:
				p.handlePrePrepare(&msg.PrePrepare)
			case msg := <-p.prepareChan:
				p.handlePrepare(&msg.Prepare)
			case msg := <-p.commitChan:
				p.handleCommit(&msg.Commit)
			}
		}
	}
}

// PrePrepare intializes a full run of the protocol
func (p *Protocol) PrePrepare() error {
	// pre-prepare: broadcast the block
	var err error
	dbg.Print(p.Node.Name(), "Broadcast PrePrepare")
	p.broadcast(func(tn *sda.TreeNode) {
		prep := PrePrepare{p.trBlock}
		tempErr := p.Node.SendTo(tn, &prep)
		if tempErr != nil {
			err = tempErr
		}
	})
	dbg.Print(p.Node.Name(), "Broadcast PrePrepare DONE")
	//p.handlePrePrepare(&PrePrepare{p.trBlock})
	return err
}

func (p *Protocol) handlePrePrepare(prePre *PrePrepare) {
	// prepare: verify the structure of the block and broadcast
	// prepare msg (with header hash of the block)
	dbg.Print(p.Node.Name(), "handlePrePrepare() BROADCASTING PREPARE msg")
	var err error
	if verifyBlock(prePre.TrBlock, "", "") {
		p.broadcast(func(tn *sda.TreeNode) {
			prep := Prepare{prePre.TrBlock.HeaderHash}
			dbg.Print(p.Node.Name(), "Sending PREPARE to", tn.Name(), "msg", prep)
			tempErr := p.Node.SendTo(tn, &prep)
			if tempErr != nil {
				err = tempErr
			}
		})
		dbg.Lvl3(p.Node.Name(), "handlePrePrepare() BROADCASTING PREPARE msgs DONE")
	} else {
		dbg.Print("Block couldn't be verified")
	}
	if err != nil {
		dbg.Error("Error while broadcasting Prepare msg", err)
	}
}

func (p *Protocol) handlePrepare(pre *Prepare) {
	p.prepMsgCount++
	dbg.Lvl4(p.Node.Name(), "We got", p.prepMsgCount,
		"Prepare msgs and threshold is", p.threshold)
	if p.prepMsgCount >= p.threshold {
		dbg.Lvl3(p.Node.Name(), "Threshold reached: broadcast Commit")
		// reset counter
		p.prepMsgCount = 1
		var err error
		p.broadcast(func(tn *sda.TreeNode) {
			com := Commit{pre.HeaderHash}
			tempErr := p.Node.SendTo(tn, &com)
			if tempErr != nil {
				dbg.Error("Error while broadcasting Commit msg", tempErr)
				err = tempErr
			}
		})
		if err != nil {
			dbg.Error("Error while broadcasting Commit msg", err)
		}
	}
}

func (p *Protocol) handleCommit(com *Commit) {
	// finish after threshold of Commit msgs
	p.commitMsgCount++
	dbg.Lvl4(p.Node.Name(), "----------------\nWe got", p.commitMsgCount,
		"COMMIT msgs and threshold is", p.threshold)
	if p.IsRoot() {
		dbg.Lvl4("Leader got ", p.commitMsgCount)
	}
	if p.commitMsgCount >= p.threshold {
		// reset counter
		p.commitMsgCount = 1
		dbg.Lvl3(p.Node.Name(), "Threshold reached: We are done... CONSENSUS")
		if p.IsRoot() && p.onDoneCB != nil {
			dbg.Lvl3(p.Node.Name(), "We are root and threshold reached: return to the simulation.")
			p.onDoneCB()
		}
		//	p.Done()
		return

	}
}

// sendCb should contain the real sendTo call and the msg to broadcast
// example for sendCb:
// func(tn *sda.TreeNode) { p.Node.SendTo(tn, &registerdMsg )}
func (p *Protocol) broadcast(sendCb func(*sda.TreeNode)) {
	for i, tn := range p.nodeList {
		if i == p.index {
			dbg.Print(p.Node.Name(), "index", p.index)
			continue
		}
		sendCb(tn)
	}
}

// verifyBlock is a simulation of a real block verification algorithm
// FIXME merge with Nicolas' code (public method in bizcoin)
func verifyBlock(block *blockchain.TrBlock, lastBlock, lastKeyBlock string) bool {
	//We measure the average block verification delays is 174ms for an average
	//block of 500kB.
	//To simulate the verification cost of bigger blocks we multiply 174ms
	//times the size/500*1024
	b, _ := json.Marshal(block)
	s := len(b)
	var n time.Duration
	n = time.Duration(s / (500 * 1024))
	time.Sleep(150 * time.Millisecond * n) //verification of 174ms per 500KB simulated
	// verification of the header
	verified := block.Header.Parent == lastBlock && block.Header.ParentKey == lastKeyBlock
	verified = verified && block.Header.MerkleRoot == blockchain.HashRootTransactions(block.TransactionList)
	verified = verified && block.HeaderHash == blockchain.HashHeader(block.Header)

	return verified
}
