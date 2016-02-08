package bizcoin

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/sda"
)

type BizCoin struct {
	// prepare-round
	prepare cosi.Cosi
	// commit-round
	commit cosi.Cosi
}

func NewBizCoinProtocol(n *sda.Node) (*BizCoin, error) {

}
func (bz *BizCoin) Start() error {

}

func (bz *BizCoin) Dispatch() error {

}
