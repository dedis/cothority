package byzcoin_ntree

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin"
)

type NtreeServer struct {
	*byzcoin.ByzCoinServer
}

func NewNtreeServer(blockSize int) *NtreeServer {
	ns := new(NtreeServer)
	// we dont care about timeout + fail in Naive comparison
	ns.ByzCoinServer = byzcoin.NewByzCoinServer(blockSize, 0, 0)
	return ns
}

func (nt *NtreeServer) Instantiate(node *sda.Node) (sda.ProtocolInstance, error) {
	dbg.Lvl2("NtreeServer waiting enough transactions...")
	currTransactions := nt.WaitEnoughBlocks()
	pi, err := NewNTreeRootProtocol(node, currTransactions)
	node.SetProtocolInstance(pi)
	dbg.Lvl1("NtreeServer instantiated Ntree Root Protocol with", len(currTransactions), " transactions")
	return pi, err
}
