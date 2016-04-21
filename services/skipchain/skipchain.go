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
	c         sda.Context
	path      string
}

// ProcessRequest treats external request to this service.
func (s *Service) ProcessClientRequest(e *network.Entity, cr *sda.ClientRequest) {
	var reply network.ProtocolMessage
	if err := s.c.SendRaw(e, reply); err != nil {
		dbg.Error(err)
	}
}

// ProcessServiceMessage is to implement the Service interface.
func (cs *Service) ProcessServiceMessage(e *network.Entity, s *sda.ServiceMessage) {
	return
}

func (cs *Service) AddSkipBlock(e *network.Entity, msg *AddSkipBlock) (network.ProtocolMessage, error) {
	return nil, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (c *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1("Cosi Service received New Protocol event")
	pi, err := cosi.NewProtocolCosi(tn)
	go pi.Dispatch()
	return pi, err
}

func newSkipchainService(c sda.Context, path string) sda.Service {
	s := &Service{
		c:    c,
		path: path,
	}
	//s.AddMessage(s.AddSkipBlock)
	return s
}
