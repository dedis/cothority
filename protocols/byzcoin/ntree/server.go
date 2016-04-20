package byzcoinNtree

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin"
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
	dbg.Lvl2("Waiting for enough transactions...")
	currTransactions := nt.WaitEnoughBlocks()
	pi, err := NewNTreeRootProtocol(node, currTransactions)
	dbg.Lvl2("Instantiated Ntree Root Protocol with", len(currTransactions), "transactions")
	return pi, err
}
