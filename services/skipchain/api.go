package skipchain

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*sda.Client
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: sda.NewClient("Skipchain")}
}

// AddSkipBlock takes a previous and a new skipchain and sends it to the
// first TreeNodeEntity
func (sc *Client) AddSkipBlock(prev, new *SkipBlock) (*AddRet, error) {
	dbg.LLvl3("Adding a new skipblock", new)
	if new.Tree == nil {
		return nil, errors.New("No tree given")
	}
	nodes := new.Tree.List()
	if len(nodes) == 0 {
		return nil, errors.New("Need at least one node in the Cothority")
	}
	dst := nodes[0].Entity
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
		New:          &sb2,
		NewTree:      new.Tree.MakeTreeMarshal(),
	}
	dbg.LLvl4("Sending message to", dst)
	reply, err := sc.Send(dst, msg)
	if e := sda.ErrMsg(reply, err); e != nil {
		dbg.Print("err")
		return nil, e
	}
	aar, ok := reply.Msg.(AddRet)
	if !ok {
		return nil, errors.New("Couldn't cast reply to AddRet")
	}
	return &aar, nil
}
