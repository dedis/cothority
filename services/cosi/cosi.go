package service

import (
	"errors"

	"fmt"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/cosi"
	"github.com/dedis/cothority/sda"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

// ServiceName is the name to refer to the CoSi service
const ServiceName = "CoSi"

func init() {
	sda.RegisterNewService(ServiceName, newCoSiService)
	network.RegisterPacketType(&SignatureRequest{})
	network.RegisterPacketType(&SignatureResponse{})
}

// CoSi is the service that handles collective signing operations
type CoSi struct {
	*sda.ServiceProcessor
	path string
}

// SignatureRequest is what the Cosi service is expected to receive from clients.
type SignatureRequest struct {
	Message []byte
	Roster  *sda.Roster
}

// SignatureResponse is what the Cosi service will reply to clients.
type SignatureResponse struct {
	Sum       []byte
	Signature []byte
}

// SignatureRequest treats external request to this service.
func (cs *CoSi) SignatureRequest(si *network.ServerIdentity, req *SignatureRequest) (network.Body, error) {
	tree := req.Roster.GenerateBinaryTree()
	tni := cs.NewTreeNodeInstance(tree, tree.Root, protocol.Name)
	pi, err := protocol.NewCoSi(tni)
	if err != nil {
		return nil, errors.New("Couldn't make new protocol: " + err.Error())
	}
	cs.RegisterProtocolInstance(pi)
	pcosi := pi.(*protocol.CoSi)
	pcosi.SigningMessage(req.Message)
	h, err := crypto.HashBytes(network.Suite.Hash(), req.Message)
	if err != nil {
		return nil, errors.New("Couldn't hash message: " + err.Error())
	}
	response := make(chan []byte)
	pcosi.RegisterSignatureHook(func(sig []byte) {
		response <- sig
	})
	log.Lvl3("Cosi Service starting up root protocol")
	go pi.Dispatch()
	go pi.Start()
	sig := <-response
	if log.DebugVisible() > 1 {
		fmt.Printf("%s: Signed a message.\n", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
	}
	return &SignatureResponse{
		Sum:       h,
		Signature: sig,
	}, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (cs *CoSi) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl3("Cosi Service received New Protocol event")
	pi, err := protocol.NewCoSi(tn)
	go pi.Dispatch()
	return pi, err
}

func newCoSiService(c *sda.Context, path string) sda.Service {
	s := &CoSi{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}
	err := s.RegisterMessage(s.SignatureRequest)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message:")
	}
	return s
}
