package skipchain

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*sda.Client
}

// NewSkipchainClient instantiates a new client with name 'n'
func NewSkipchainClient() *Client {
	return &Client{Client: sda.NewClient("Skipchain")}
}

// ActiveAdd takes a previous and a new skipchain and sends it to the
// first TreeNodeEntity
func (sc *Client) ActiveAdd(prev, new *SkipBlock) (*AddRet, error) {
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
	var tm1 *sda.TreeMarshal
	if prev != nil {
		sb1 = *prev
		sb1.Tree = nil
		tm1 = prev.Tree.MakeTreeMarshal()
	}
	sb2 := *new
	sb2.Tree = nil
	msg := &AddSkipBlock{
		Previous:     &sb1,
		PreviousTree: tm1,
		New: &sb2,
		NewTree: new.Tree.MakeTreeMarshal(),
	}
	b, err := network.MarshalRegisteredType(msg)
	dbg.Print("dbg")
	if err != nil {
		return nil, err
	}
	dbg.Print("dbg")
	dbg.LLvl4("Sending message to", dst)
	dbg.Print("dbg")
	reply, err := sc.Send(dst, b)
	dbg.Print("dbg")
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	aar, ok := reply.Msg.(AddRet)
	if !ok {
		return nil, errors.New("Couldn't cast reply to AddRet")
	}
	return &aar, nil
}
