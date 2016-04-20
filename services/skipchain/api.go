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
	if new.Tree == nil {
		return nil, errors.New("No tree given")
	}
	nodes := new.Tree.List()
	if len(nodes) == 0 {
		return nil, errors.New("Need at least one node in the Cothority")
	}
	dst := nodes[0].Entity
	b, err := network.MarshalRegisteredType(&ActiveAdd{prev, new})
	if err != nil {
		return nil, err
	}
	dbg.LLvl4("Sending message to", dst)
	msg, err := sc.Send(dst, b)
	if err != nil {
		return nil, err
	}
	aar, ok := msg.Msg.(ActiveAddRet)
	if !ok {
		return nil, ErrMsg(msg, err)
	}
	return &aar, nil
}
