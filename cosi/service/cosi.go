package service

import (
	"errors"
	"time"

	"github.com/dedis/cothority/cosi/protocol"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

// ServiceName is the name to refer to the CoSi service
const ServiceName = "CoSiService"

func init() {
	onet.RegisterNewService(ServiceName, newCoSiService)
	network.RegisterMessage(&SignatureRequest{})
	network.RegisterMessage(&SignatureResponse{})
}

// Service is the service that handles collective signing operations
type Service struct {
	*onet.ServiceProcessor
}

// SignatureRequest is what the Cosi service is expected to receive from clients.
type SignatureRequest struct {
	Message []byte
	Roster  *onet.Roster
}

// SignatureResponse is what the Cosi service will reply to clients.
type SignatureResponse struct {
	Hash      []byte
	Signature []byte
}

// SignatureRequest treats external request to this service.
func (s *Service) SignatureRequest(req *SignatureRequest) (network.Message, error) {
	// generate the tree
	nNodes := len(req.Roster.List)
	tree := req.Roster.GenerateNaryTreeWithRoot(nNodes, s.ServerIdentity())
	if tree == nil {
		return nil, errors.New("failed to generate tree")
	}
	pi, err := s.CreateProtocol(protocol.ProtocolName, tree)
	if err != nil {
		return nil, errors.New("Couldn't make new protocol: " + err.Error())
	}

	// configure the protocol
	p := pi.(*protocol.CoSiRootNode)
	p.CreateProtocol = s.CreateProtocol
	p.Proposal = req.Message
	// TODO is there an optimal way to find out the number of subtrees?
	p.NSubtrees = nNodes / 10
	if p.NSubtrees < 1 {
		p.NSubtrees = 1
	}

	// start the protocol
	log.Lvl3("Cosi Service starting up root protocol")
	if err = pi.Start(); err != nil {
		return nil, err
	}

	if log.DebugVisible() > 1 {
		log.Printf("%s: Signed a message.\n", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
	}

	// wait for reply
	var sig []byte
	select {
	case sig = <-p.FinalSignature:
	case <-time.After(protocol.DefaultProtocolTimeout + time.Second):
		return nil, errors.New("protocol timed out")
	}

	// the hash is the message cosi actually signs, ideally cosi protocol
	// should tell us what it is, here we recompute it and then return
	suite, ok := s.Suite().(kyber.HashFactory)
	if !ok {
		return nil, errors.New("suite is unusable")
	}
	h := suite.Hash()
	h.Write(req.Message)
	return &SignatureResponse{h.Sum(nil), sig}, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Cosi Service received New Protocol event")
	pi, err := protocol.NewProtocol(tn)
	return pi, err
}

func newCoSiService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandler(s.SignatureRequest); err != nil {
		log.Error("couldn't register message:", err)
		return nil, err
	}
	if _, err := c.ProtocolRegister(protocol.ProtocolName, protocol.NewProtocol); err != nil {
		log.Error("couldn't register protocol:", err)
		return nil, err
	}
	return s, nil
}
