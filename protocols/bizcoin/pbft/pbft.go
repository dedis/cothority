package pbft

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

// Protocol implements sda.Protocol
type Protocol struct {
	// the node we are represented-in
	*sda.Node
	// the suite we use
	suite abstract.Suite
	// aggregated public key of the peers
	aggregatedPublic abstract.Point

	// channels:
	prePrepareChan chan prePrepareChan
	prepareChan    chan prepareChan
	commitChan     chan commitChan
}

func NewProtocol(n *sda.Node) (*Protocol, error) {
	pbft := new(Protocol)
	n.RegisterChannel(&pbft.prePrepareChan)
	n.RegisterChannel(&pbft.prepareChan)
	n.RegisterChannel(&pbft.commitChan)
	return pbft, nil
}

func NewRootProtocol(n *sda.Node) (*Protocol, error) {
	pbft := new(Protocol)

	return pbft, nil
}

// Start will start pre-prepare and TODO ???
func (p *Protocol) Start() error {

	return nil
}

// Dispatch listen on the different channels
func (p *Protocol) Dispatch() error {
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

func (p *Protocol) handlePrePrepare(prePre *PrePrepare) {

}

func (p *Protocol) handlePrepare(pre *Prepare) {

}

func (p *Protocol) handleCommit(com *Commit) {

}

// TODO
// pre-prepare: send around block,
// prepare: verify the structure of the block and broadcast prepare msg (with header hash of the block)
// commit: count prep msgs, if > threshold -> broadcast commit msg (hash of the block header)
// we don't do a MAC like in the PBFT paper but a hash (or even a signature)
