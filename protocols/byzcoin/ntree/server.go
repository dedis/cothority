package byzcoinNtree

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/byzcoin"
	"github.com/dedis/cothority/sda"
)

// NtreeServer is similar to byzcoin.Server
type NtreeServer struct {
	*byzcoin.Server
}

// NewNtreeServer returns a new block server for Ntree
func NewNtreeServer(blockSize int) *NtreeServer {
	ns := new(NtreeServer)
	// we don't care about timeout + fail in Naive comparison
	ns.Server = byzcoin.NewByzCoinServer(blockSize, 0, 0)
	return ns
}

// Instantiate returns a new NTree protocol instance
func (nt *NtreeServer) Instantiate(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	log.Lvl2("Waiting for enough transactions...")
	currTransactions := nt.WaitEnoughBlocks()
	pi, err := NewNTreeRootProtocol(node, currTransactions)
	log.Lvl2("Instantiated Ntree Root Protocol with", len(currTransactions), "transactions")
	return pi, err
}
