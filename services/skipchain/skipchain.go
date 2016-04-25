package skipchain

import (
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
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
	*ServiceProcessor
	path string
}

func (s *Service) ProposeSkipBlock(latest crypto.HashID, proposed SkipBlock) (*ProposedSkipBlockReply, error) {
	return nil, nil
}

func (s *Service) GetUpdateChain(latest crypto.HashID) (*GetUpdateChainReply, error) {
	return nil, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1("SkipChain received New Protocol event", tn, conf)
	return nil, nil
}

// verifyNewSkipBlock calls the appropriate app-verification and returns
// either a signature on the newest SkipBlock or nil if the SkipBlock
// has been refused
func (s *Service) verifyNewSkipBlock(latest, newest *SkipBlockCommon) bool {
	// TODO: implement a protocol that can check on the veracity of the new
	// TODO: EntityList
	return true
}

func newSkipchainService(c sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: NewProcessor(c),
		path:             path,
	}
	return s
}
