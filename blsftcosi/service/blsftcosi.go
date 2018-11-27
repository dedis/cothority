// Package service implements a blsftcosi service for which clients can connect to
// and then sign messages.
package service

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/dedis/cothority/blsftcosi/protocol"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

const propagationTimeout = 2 * time.Second
const protocolTimeout = 10 * time.Second

// ServiceID is
var ServiceID onet.ServiceID

// ServiceName is the name to refer to the CoSi service
const ServiceName = "blsftCoSiService"

func init() {
	ServiceID, _ = onet.RegisterNewService(ServiceName, newCoSiService)
	network.RegisterMessage(&SignatureRequest{})
	network.RegisterMessage(&SignatureResponse{})
}

// Service is the service that handles collective signing operations
type Service struct {
	*onet.ServiceProcessor
	suite     pairing.Suite
	wg        sync.WaitGroup
	Threshold int
	NSubtrees int
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

	// configure the BlsFtCosi protocol
	pi, err := s.CreateProtocol(protocol.DefaultProtocolName, tree)
	if err != nil {
		return nil, errors.New("Couldn't make new protocol: " + err.Error())
	}
	p := pi.(*protocol.BlsFtCosi)
	p.CreateProtocol = s.CreateProtocol
	p.Msg = req.Message
	// We set NSubtrees to the square root of n to evenly distribute the load
	if s.NSubtrees <= 0 {
		p.SetNbrSubTree(int(math.Sqrt(float64(nNodes))))
	} else {
		p.SetNbrSubTree(s.NSubtrees)
	}
	p.Timeout = protocolTimeout

	// Complete Threshold
	p.Threshold = p.Tree().Size()
	if s.Threshold > 0 {
		p.Threshold = s.Threshold
	}

	// start the protocol
	log.Lvl3("Cosi Service starting up root protocol")
	if err = pi.Start(); err != nil {
		return nil, err
	}

	// wait for reply
	var sig protocol.BlsSignature
	select {
	case sig = <-p.FinalSignature:
	case <-time.After(p.Timeout + time.Second):
		return nil, errors.New("protocol timed out")
	}

	// The hash is the message ftcosi actually signs, we recompute it the
	// same way as ftcosi and then return it.
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
		log.Lvl3("IT SHOULD NEVER COME HERE")
		pi, err := protocol.NewDefaultProtocol(tn)
		if err != nil {
			return nil, err
		}
		blsftcosi := pi.(*protocol.BlsFtCosi)
		return blsftcosi, nil
	case protocol.DefaultSubProtocolName:
		pi, err := protocol.NewDefaultSubProtocol(tn)
		if err != nil {
			return nil, err
		}
		subblsftcosi := pi.(*protocol.SubBlsFtCosi)
		return subblsftcosi, nil
	}
	return nil, errors.New("no such protocol " + tn.ProtocolName())
}

func newCoSiService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		suite:            bn256.NewSuiteG2(),
	}
	s.wg.Add(1)

	if err := s.RegisterHandler(s.SignatureRequest); err != nil {
		log.Error("couldn't register message:", err)
		return nil, err
	}

	return s, nil
}
