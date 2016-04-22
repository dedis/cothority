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

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	network.RegisterMessageType(&RequestNewBlock{})
	return &Client{Client: sda.NewClient("Skipchain")}
}

// RequestNewBlock sends an EntityList to the SkipChain and will ask
// the application 'app' to verify the new EntityList.
func (sc *Client) RequestNewBlock(app string, el *sda.EntityList) (*RNBRet, error) {
	dbg.Lvl3("Adding a new skipblock", el)
	if len(el.List) == 0 {
		return nil, errors.New("Need at least one node in the Cothority")
	}

	msg := &RequestNewBlock{
		AppId:      app,
		EntityList: el,
	}

	dbg.Lvl4("Sending message to", el.List[0])
	reply, err := sc.Send(el.List[0], msg)

	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	ret, ok := reply.Msg.(RNBRet)
	if !ok {
		return nil, errors.New("Couldn't cast reply to AddRet")
	}
	return &ret, nil
}
