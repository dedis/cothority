package status

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

// This file contains all the code to run a Stat service. It is used to reply to
// client request for status.
// It would be very easy to write an
// updated version that provides additional data

// ServiceName is the name to refer to the Status service
const ServiceName = "Status"

func init() {
	sda.RegisterNewService(ServiceName, newStatService)
	network.RegisterMessageType(&StatusRequest{})
	network.RegisterMessageType(&StatusResponse{})

}

// Stat is the service that handles collective signing operations
type Stat struct {
	*sda.ServiceProcessor
	path string
}

// SignatureRequest is what the Cosi service is expected to receive from clients.
type StatusRequest struct{}

// StatRequestType is the type that is embedded in the Request object for a
// StatRequest
var StatRequestType = network.RegisterMessageType(StatusRequest{})

// SignatureResponse is what the Cosi service will reply to clients.
type StatusResponse struct {
	Connections int
}

// StatResponseType is the type that is embedded in the Request object for a
// StatResponse
var StatResponseType = network.RegisterMessageType(StatusResponse{})

// SignatureRequest treats external request to this service.
func (st *Stat) StatusRequest(e *network.Entity, req *StatusRequest) (network.ProtocolMessage, error) {
	return &StatusResponse{
		Connections: len(st.Context.(*sda.DefaultContext).Host.Connections),
	}, nil
}


func newStatService(c sda.Context, path string) sda.Service {
	s := &Stat{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}
	err := s.RegisterMessage(s.StatusRequest)
	if err != nil {
		dbg.ErrFatal(err, "Couldn't register message:")
	}
	return s
}
func (cs *Stat) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}