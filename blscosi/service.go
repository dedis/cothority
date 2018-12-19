// Package blscosi implements a service and client that provides an API
// to request a signature to a cothority
package blscosi

import (
	"errors"
	"time"

	"github.com/dedis/cothority/blscosi/protocol"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

const protocolTimeout = 10 * time.Second

var suite = suites.MustFind("bn256.adapter").(*pairing.SuiteBn256)

// ServiceID is the key to get the service later
var ServiceID onet.ServiceID

// ServiceName is the name to refer to the CoSi service
const ServiceName = "blsCoSiService"

func init() {
	ServiceID, _ = onet.RegisterNewServiceWithSuite(ServiceName, suite, newCoSiService)
	network.RegisterMessage(&SignatureRequest{})
	network.RegisterMessage(&SignatureResponse{})
}

// Service is the service that handles collective signing operations
type Service struct {
	*onet.ServiceProcessor
	suite     pairing.Suite
	Threshold int
	NSubtrees int
	Timeout   time.Duration
}

// SignatureRequest is what the Cosi service is expected to receive from clients.
type SignatureRequest struct {
	Message []byte
	Roster  *onet.Roster
}

// SignatureResponse is what the Cosi service will reply to clients.
type SignatureResponse struct {
	Hash      []byte
	Signature protocol.BlsSignature
}

// SignatureRequest treats external request to this service.
func (s *Service) SignatureRequest(req *SignatureRequest) (network.Message, error) {
	// generate the tree
	nNodes := len(req.Roster.List)
	rooted := req.Roster.NewRosterWithRoot(s.ServerIdentity())
	if rooted == nil {
		return nil, errors.New("we're not in the roster")
	}
	tree := rooted.GenerateNaryTree(nNodes)
	if tree == nil {
		return nil, errors.New("failed to generate tree")
	}

	// configure the BlsCosi protocol
	pi, err := s.CreateProtocol(protocol.DefaultProtocolName, tree)
	if err != nil {
		return nil, errors.New("Couldn't make new protocol: " + err.Error())
	}
	p := pi.(*protocol.BlsCosi)
	p.CreateProtocol = s.CreateProtocol
	p.Timeout = s.Timeout
	p.Msg = req.Message

	// Threshold before the subtrees so that we can optimize situation
	// like a threshold of one
	if s.Threshold > 0 {
		p.Threshold = s.Threshold
	}

	if s.NSubtrees > 0 {
		err = p.SetNbrSubTree(s.NSubtrees)
		if err != nil {
			return nil, err
		}
	}

	// start the protocol
	log.Lvl3("Cosi Service starting up root protocol")
	if err = pi.Start(); err != nil {
		return nil, err
	}

	// wait for reply. This will always eventually return.
	sig := <-p.FinalSignature

	// The hash is the message blscosi actually signs, we recompute it the
	// same way as blscosi and then return it.
	h := s.suite.Hash()
	h.Write(req.Message)
	return &SignatureResponse{h.Sum(nil), sig}, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Cosi Service received on", s.ServerIdentity(), "received new protocol event-", tn.ProtocolName())
	switch tn.ProtocolName() {
	case protocol.DefaultProtocolName:
		return protocol.NewDefaultProtocol(tn)
	case protocol.DefaultSubProtocolName:
		return protocol.NewDefaultSubProtocol(tn)
	}
	return nil, errors.New("no such protocol " + tn.ProtocolName())
}

func newCoSiService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		suite:            suite,
		Timeout:          protocolTimeout,
	}

	if err := s.RegisterHandler(s.SignatureRequest); err != nil {
		log.Error("couldn't register message:", err)
		return nil, err
	}

	return s, nil
}
