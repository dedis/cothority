package skipchain

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/cosi"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

func init() {
	sda.RegisterNewService("Skipchain", newSkipchainService)
}

// Service handles adding new SkipBlocks
type Service struct {
	*Processor
	path string
}

// RequestNewBlock receives a new EntityList and will call the appropriate
// application-protocol to verify if the new EntityList should be included
// in the SkipChain.
func (cs *Service) RequestNewBlock(e *network.Entity, msg *RequestNewBlock) (network.ProtocolMessage, error) {
	sb := NewSkipBlock(msg.EntityList)
	ar := &AddRet{
		SkipBlock: sb,
	}
	return ar, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (c *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1("SkipChain received New Protocol event", tn, conf)
	pi, err := cosi.NewProtocolCosi(tn)
	go pi.Dispatch()
	return pi, err
}

func newSkipchainService(c sda.Context, path string) sda.Service {
	s := &Service{
		Processor: NewProcessor(c),
		path:      path,
	}
	s.AddMessage(s.RequestNewBlock)
	return s
}
