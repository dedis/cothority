// Package service implements a cosi service for which clients can connect to
// and then sign messages.
package service

import (
	"errors"
	"fmt"
	"time"

	"go.dedis.ch/cothority/v3/cosi/protocol"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"gopkg.in/satori/go.uuid.v1"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

// ServiceName is the name to refer to the CoSi service
const ServiceName = "CoSi"

func init() {
	onet.RegisterNewService(ServiceName, newCoSiService)
	network.RegisterMessage(&SignatureRequest{})
	network.RegisterMessage(&SignatureResponse{})
}

// CoSi is the service that handles collective signing operations
type CoSi struct {
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
func (cs *CoSi) SignatureRequest(req *SignatureRequest) (network.Message, error) {
	suite, ok := cs.Suite().(kyber.HashFactory)
	if !ok {
		return nil, errors.New("suite is unusable")
	}

	if req.Roster.ID.IsNil() {
		req.Roster.ID = onet.RosterID(uuid.NewV4())
	}

	_, root := req.Roster.Search(cs.ServerIdentity().ID)
	if root == nil {
		return nil, errors.New("Couldn't find a serverIdetity in Roster")
	}
	tree := req.Roster.GenerateNaryTreeWithRoot(2, root)
	tni := cs.NewTreeNodeInstance(tree, tree.Root, cosi.Name)
	pi, err := cosi.NewProtocol(tni)
	if err != nil {
		return nil, errors.New("Couldn't make new protocol: " + err.Error())
	}
	cs.RegisterProtocolInstance(pi)
	pcosi := pi.(*cosi.CoSi)
	pcosi.SigningMessage(req.Message)
	h := suite.Hash()
	h.Write(req.Message)
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
		Hash:      h.Sum(nil),
		Signature: sig,
	}, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (cs *CoSi) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Cosi Service received New Protocol event")
	pi, err := cosi.NewProtocol(tn)
	return pi, err
}

func newCoSiService(c *onet.Context) (onet.Service, error) {
	s := &CoSi{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	err := s.RegisterHandler(s.SignatureRequest)
	if err != nil {
		log.Error(err, "Couldn't register message:")
		return nil, err
	}
	return s, nil
}
