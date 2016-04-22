package skipchain

import (
	"errors"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
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
	if msg.SkipBlock.Index == 0 {
		// Create Genesis SkipBlock
		sb.Index = 1
		sb.BackLink = [][]byte{[]byte("Genesis")}
	} else {
		// Create SkipBlock with back-links
	}
	if !cs.verifyNewSkipBlock(msg.AppId, msg.SkipBlock, sb) {
		return nil, errors.New("New EntityList has been rejected")
	}

	ar := &RNBRet{
		SkipBlock: sb,
	}
	return ar, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (c *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1("SkipChain received New Protocol event", tn, conf)
	return nil, nil
}

// verifyNewSkipBlock calls the appropriate app-verification and returns
// either a signature on the newest SkipBlock or nil if the SkipBlock
// has been refused
func (c *Service) verifyNewSkipBlock(app string, last, newest *SkipBlock) bool {
	// TODO: implement a protocol that can check on the veracity of the new
	// TODO: EntityList
	accepted := app == "accept"
	if accepted{
		newest.Signature = cosi.NewSignature(network.Suite)
	}
	return accepted
}

func newSkipchainService(c sda.Context, path string) sda.Service {
	s := &Service{
		Processor: NewProcessor(c),
		path:      path,
	}
	s.AddMessage(s.RequestNewBlock)
	return s
}
