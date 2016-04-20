package skipchain

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
)

// SkipchainClient is a structure to communicate with the Skipchain
// service from the outside
type SkipchainClient struct {
	*Client
}

// NewSkipchainClient instantiates a new client with name 'n'
func NewSkipchainClient() *SkipchainClient {
	return &SkipchainClient{Client: NewClient("Skipchain")}
}

// SendActiveAdd takes a previous and a new skipchain and sends it to the
// first TreeNodeEntity
func (sc *SkipchainClient) ActiveAdd(prev, new *SkipBlock) (*ActiveAddRet, error) {
	dbg.LLvl3("Adding a new skipblock", new)
	dbg.Print("dbg")
	if new.Tree == nil {
		return nil, errors.New("No tree given")
	}
	dbg.Print("dbg")
	nodes := new.Tree.List()
	dbg.Print("dbg")
	if len(nodes) == 0 {
		return nil, errors.New("Need at least one node in the Cothority")
	}
	dbg.Print("dbg")
	dst := nodes[0].Entity
	dbg.Print("dbg")
	var sb1 SkipBlock
	if prev != nil {
		sb1 = *prev
		sb1.Tree = nil
	}
	sb2 := *new
	sb2.Tree = nil
	msg := &AddSkipBlock{
		Previous:     sb1,
		PreviousTree: prev.Tree.MakeTreeMarshal(),
	}
	b, err := network.MarshalRegisteredType(&AddSkipBlock{prev, new})
	dbg.Print("dbg")
	if err != nil {
		return nil, err
	}
	dbg.Print("dbg")
	dbg.LLvl4("Sending message to", dst)
	dbg.Print("dbg")
	msg, err := sc.Send(dst, b)
	dbg.Print("dbg")
	if err != nil {
		return nil, err
	}
	aar, ok := msg.Msg.(ActiveAddRet)
	if !ok {
		return nil, ErrMsg(msg, err)
	}
	return &aar, nil
}
