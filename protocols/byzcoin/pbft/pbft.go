// Package pbft is the Practical Byzantine Fault Tolerance algorithm with some simplifications.
package pbft

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
	"github.com/dedis/crypto/abstract"
)

const (
	notFound = -1
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
	*sda.TreeNodeInstance
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

	state int

	tempPrepareMsg []*Prepare
	tempCommitMsg  []*Commit

	finishChan chan finishChan
}

const (
	statePrePrepare = iota
	statePrepare
	stateCommit
	stateFinished
)

// NewProtocol returns a new pbft protocol
func NewProtocol(n *sda.TreeNodeInstance) (*Protocol, error) {
	pbft := new(Protocol)
	pbft.state = statePrePrepare
	tree := n.Tree()
	pbft.TreeNodeInstance = n
	pbft.nodeList = tree.List()
	idx := notFound
	for i, tn := range pbft.nodeList {
		if tn.Id.Equals(n.TreeNode().Id) {
			idx = i
		}
	}
	if idx == notFound {
		panic(fmt.Sprintf("Could not find ourselves %+v in the list of nodes %+v", n, pbft.nodeList))
	}
	pbft.index = idx
	// 2/3 * #participants == threshold FIXME the threshold is actually XXX
	pbft.threshold = int(math.Ceil(float64(len(pbft.nodeList)) * 2.0 / 3.0))
	pbft.prepMsgCount = 0
	pbft.commitMsgCount = 0

	if err := n.RegisterChannel(&pbft.prePrepareChan); err != nil {
		return pbft, err
	}
	if err := n.RegisterChannel(&pbft.prepareChan); err != nil {
		return pbft, err
	}
	if err := n.RegisterChannel(&pbft.commitChan); err != nil {
		return pbft, err
	}
	if err := n.RegisterChannel(&pbft.finishChan); err != nil {
		return pbft, err
	}

	return pbft, nil
}

// Dispatch implements sda.Protocol (and listens on all message channels)
func (p *Protocol) Dispatch() error {
	for {
		select {
		case msg := <-p.prePrepareChan:
			p.handlePrePrepare(&msg.PrePrepare)
		case msg := <-p.prepareChan:
			p.handlePrepare(&msg.Prepare)
		case msg := <-p.commitChan:
			p.handleCommit(&msg.Commit)
		case <-p.finishChan:
			dbg.Lvl3(p.Name(), "Got Done Message ! FINISH")
			p.Done()
			return nil
		}
	}
}

// Start implements the ProtocolInstance interface of sda.
func (p *Protocol) Start() error {
	return p.PrePrepare()
}

// PrePrepare intializes a full run of the protocol.
func (p *Protocol) PrePrepare() error {
	// pre-prepare: broadcast the block
	var err error
	dbg.Lvl2(p.Name(), "Broadcast PrePrepare")
	prep := &PrePrepare{p.trBlock}
	p.broadcast(func(tn *sda.TreeNode) {
		tempErr := p.SendTo(tn, prep)
		if tempErr != nil {
			err = tempErr
		}
		p.state = statePrepare
	})
	dbg.Lvl3(p.Name(), "Broadcast PrePrepare DONE")
	return err
}

// handlePrePrepare receive preprepare messages and go to Prepare if it received
// enough.
func (p *Protocol) handlePrePrepare(prePre *PrePrepare) {
	if p.state != statePrePrepare {
		//dbg.Lvl3(p.Name(), "DROP preprepare packet : Already broadcasted prepare")
		return
	}
	// prepare: verify the structure of the block and broadcast
	// prepare msg (with header hash of the block)
	dbg.Lvl3(p.Name(), "handlePrePrepare() BROADCASTING PREPARE msg")
	var err error
	if verifyBlock(prePre.TrBlock, "", "") {
		// STATE TRANSITION PREPREPARE => PREPARE
		p.state = statePrepare
		prep := &Prepare{prePre.TrBlock.HeaderHash}
		p.broadcast(func(tn *sda.TreeNode) {
			//dbg.Print(p.Name(), "Sending PREPARE to", tn.Name(), "msg", prep)
			tempErr := p.SendTo(tn, prep)
			if tempErr != nil {
				err = tempErr
				dbg.Error(p.Name(), "Error broadcasting PREPARE =>", err)
			}
		})
		// Already insert the previously received messages !
		go func() {
			for _, msg := range p.tempPrepareMsg {
				p.prepareChan <- prepareChan{nil, *msg}
			}
			p.tempPrepareMsg = nil
		}()
		dbg.Lvl3(p.Name(), "handlePrePrepare() BROADCASTING PREPARE msgs DONE")
	} else {
		dbg.Lvl3(p.Name(), "Block couldn't be verified")
	}
	if err != nil {
		dbg.Error("Error while broadcasting Prepare msg", err)
	}
}

func (p *Protocol) handlePrepare(pre *Prepare) {
	if p.state != statePrepare {
		//dbg.Lvl3(p.Name(), "STORE prepare packet: wrong state")
		p.tempPrepareMsg = append(p.tempPrepareMsg, pre)
		return
	}
	p.prepMsgCount++
	//dbg.Lvl3(p.Name(), "Handle Prepare", p.prepMsgCount,
	//	"msgs and threshold is", p.threshold)
	var localThreshold = p.threshold
	// we dont have a "client", the root DONT send any prepare message
	// so for the rest of the nodes the threshold is less one.
	if !p.IsRoot() {
		localThreshold--
	}
	if p.prepMsgCount >= localThreshold {
		// TRANSITION PREPARE => COMMIT
		dbg.Lvl3(p.Name(), "Threshold (", localThreshold, ") reached: broadcast Commit")
		p.state = stateCommit
		// reset counter
		p.prepMsgCount = 0
		var err error
		com := &Commit{pre.HeaderHash}
		p.broadcast(func(tn *sda.TreeNode) {
			tempErr := p.SendTo(tn, com)
			if tempErr != nil {
				dbg.Error(p.Name(), "Error while broadcasting Commit =>", tempErr)
				err = tempErr
			}
		})
		// Dispatch already the message we received earlier !
		go func() {
			for _, msg := range p.tempCommitMsg {
				p.commitChan <- commitChan{nil, *msg}
			}
			p.tempCommitMsg = nil
		}()
		// sends to the channel the already commited messages
		if err != nil {
			dbg.Error("Error while broadcasting Commit msg", err)
		}
	}
}

// handleCommit receives commit messages and signal the end if it received
// enough of it.
func (p *Protocol) handleCommit(com *Commit) {
	if p.state != stateCommit {
		//	dbg.Lvl3(p.Name(), "STORE handle commit packet")
		p.tempCommitMsg = append(p.tempCommitMsg, com)
		return
	}
	// finish after threshold of Commit msgs
	p.commitMsgCount++
	dbg.Lvl4(p.Name(), "----------------\nWe got", p.commitMsgCount,
		"COMMIT msgs and threshold is", p.threshold)
	if p.IsRoot() {
		dbg.Lvl4("Leader got ", p.commitMsgCount)
	}
	if p.commitMsgCount >= p.threshold {
		p.state = stateFinished
		// reset counter
		p.commitMsgCount = 0
		dbg.Lvl3(p.Name(), "Threshold reached: We are done... CONSENSUS")
		if p.IsRoot() && p.onDoneCB != nil {
			dbg.Lvl3(p.Name(), "We are root and threshold reached: return to the simulation.")
			p.onDoneCB()
			p.finish()
		}
		return
	}
}

// finish is called by the root to tell everyone the root is done
func (p *Protocol) finish() {
	p.broadcast(func(tn *sda.TreeNode) {
		if err := p.SendTo(tn, &Finish{"Finish"}); err != nil {
			dbg.Error(p.Name(), "couldn't send 'finish' message to",
				tn.Name(), err)
		}
	})
	// notify ourselves
	go func() { p.finishChan <- finishChan{nil, Finish{}} }()
}

// sendCb should contain the real sendTo call and the msg to broadcast
// example for sendCb:
// func(tn *sda.TreeNode) { p.SendTo(tn, &registerdMsg )}
func (p *Protocol) broadcast(sendCb func(*sda.TreeNode)) {
	for i, tn := range p.nodeList {
		if i == p.index {
			continue
		}
		go sendCb(tn)
	}
}

// verifyBlock is a simulation of a real block verification algorithm
// FIXME merge with Nicolas' code (public method in byzcoin)
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
