package bizcoin

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/sda"
)

type BizCoin struct {
	// the node we are represented-in
	*sda.Node
	// the suite we use
	suite abstract.Suite
	// prepare-round cosi
	prepare cosi.Cosi
	// commit-round cosi
	commit cosi.Cosi
	// channel for announcement
	announceChan chan announceChan
	// channel for commitment
	commitChan chan commitChan
	// channel for challenge
	challengeChan chan challengeChan
	// channel for response
	responseChan chan responseChan
}

func NewBizCoinProtocol(n *sda.Node) (*BizCoin, error) {
	// create the bizcoin
	bz := new(BizCoin)
	bz.Node = n
	bz.suite = n.Suite()
	bz.prepare = cosi.NewCosi(n.Suite(), n.Private())
	bz.commit = cosi.NewCosi(n.Suite(), n.Private())

	// register channels
	n.RegisterChannel(&bz.announceChan)
	n.RegisterChannel(&bz.commitChan)
	n.RegisterChannel(&bz.challengeChan)
	n.RegisterChannel(&bz.responseChan)

	// start listening
	go bz.listen()
	return bz
}

func (bz *BizCoin) Start() error {

}

func (bz *BizCoin) Dispatch() error {

}

func (bz *BizCoin) listen() {

}

func (bz *BizCoin) handleNewTransaction(tr blockchain.Tx) error {

}
