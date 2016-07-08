package guard

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// Client is a structure to communicate with Guard service
type Client struct {
	*sda.Client
}

// NewClient makes a new Client
func NewClient() *Client {
	return &Client{Client: sda.NewClient(ServiceName)}
}

// GetGuard Sends requests to all other members of network and creates client
//func (c *Client) GetGuard(dst *sda.Roster, UID []byte, epoch []byte, t []byte) ([]*Response, error) {
//	srs := make([]*Response, len(dst.List))
//	secrets := s.Create(1, len(dst.List), string(t))
//	//send request to all entities in the network
//	for i := 0; i < len(dst.List); i++ {
//		log.Lvl4("Sending Request to ", dst.List[i])
//		ServiceReq := &Request{UID, epoch, []byte(secrets[i])}
//		reply, err := c.Send(dst.List[i], ServiceReq)
//		if e := sda.ErrMsg(reply, err); e != nil {
//			return nil, e
//		}
//		sr, ok := reply.Msg.(Response)
//		if !ok {
//			return nil, errors.New("Wrong return type")
//		}
//		srs[i] = &sr
//	}
//	return srs, nil
//
//}

func (c *Client) GetGuard(dst *network.ServerIdentity, UID []byte, epoch []byte, t []byte) (*Response, error) {
	//send request to all entities in the network
	log.Lvl4("Sending Request to ", dst)
	ServiceReq := &Request{UID, epoch, []byte(t)}
	reply, err := c.Send(dst, ServiceReq)
	if e := sda.ErrMsg(reply, err); e != nil {
		return nil, e
	}
	sr, ok := reply.Msg.(Response)
	if !ok {
		return nil, errors.New("Wrong return type")
	}
	return &sr, nil
}
