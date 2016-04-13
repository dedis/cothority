package cosi

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/cosi"
	"github.com/dedis/crypto/abstract"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

func init() {
	sda.RegisterNewService("Cosi", newCosiService)
}

// Cosi is the service that handles collective signing operations
type Cosi struct {
	c    sda.Context
	path string
}

// Request is what the Cosi service is expected to receive from clients.
type ServiceRequest struct {
	Message    []byte
	EntityList *sda.EntityList
}

// CosiRequestType is the type that is embedded in the Request object for a
// CosiRequest
var CosiRequestType = network.RegisterMessageType(ServiceRequest{})

// Response is what the Cosi service will reply to clients.
type ServiceResponse struct {
	Challenge abstract.Secret
	Response  abstract.Secret
}

// CosiRequestType is the type that is embedded in the Request object for a
// CosiResponse
var CosiResponseType = network.RegisterMessageType(ServiceResponse{})

// ProcessRequest treats external request to this service.
func (cs *Cosi) ProcessRequest(e *network.Entity, r *sda.Request) {
	if r.Type != CosiRequestType {
		return
	}
	var req ServiceRequest
	// XXX should provide a UnmarshalRegisteredType(buff) func instead of having to give
	// the constructors with the suite.
	id, pm, err := network.UnmarshalRegisteredType(r.Data, network.DefaultConstructors(network.Suite))
	if err != nil {
		dbg.Error(err)
		return
	}
	if id != CosiRequestType {
		dbg.Error("Wrong message coming in")
		return
	}
	req = pm.(ServiceRequest)
	tree := req.EntityList.GenerateBinaryTree()
	tni := cs.c.NewTreeNodeInstance(tree, tree.Root)
	pi, err := cosi.NewProtocolCosi(tni)
	if err != nil {
		return
	}
	cs.c.RegisterProtocolInstance(pi)
	pcosi := pi.(*cosi.ProtocolCosi)
	pcosi.SigningMessage(req.Message)
	pcosi.RegisterDoneCallback(func(chall abstract.Secret, resp abstract.Secret) {
		respMessage := &ServiceResponse{
			Challenge: chall,
			Response:  resp,
		}
		if err := cs.c.SendRaw(e, respMessage); err != nil {
			dbg.Error(err)
		}
	})
	dbg.Lvl1("Cosi Service starting up root protocol")
	go pi.Dispatch()
	go pi.Start()
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (c *Cosi) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1("Cosi Service received New Protocol event")
	pi, err := cosi.NewProtocolCosi(tn)
	go pi.Dispatch()
	return pi, err
}

func newCosiService(c sda.Context, path string) sda.Service {
	return &Cosi{
		c:    c,
		path: path,
	}
}
