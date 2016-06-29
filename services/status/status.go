package status

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// This file contains all the code to run a Stat service. It is used to reply to
// client request for status.
// It would be very easy to write an
// updated version that provides additional data

// ServiceName is the name to refer to the Status service
const ServiceName = "Status"

func init() {
	sda.RegisterNewService(ServiceName, newStatService)
	network.RegisterMessageType(&Request{})
	network.RegisterMessageType(&Response{})

}

// Stat is the service that returns the status reports of all services running on a server.
type Stat struct {
	*sda.ServiceProcessor
	path string
}

// Request is what the Cosi service is expected to receive from clients.
type Request struct{}

// Response is what the Cosi service will reply to clients.
type Response struct {
	Msg map[string]sda.Status
}

// Request treats external request to this service.
func (st *Stat) Request(e *network.ServerIdentity, req *Request) (network.Body, error) {
	return &Response{st.Context.ReportStatus()}, nil
}

func newStatService(c *sda.Context, path string) sda.Service {
	s := &Stat{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}
	err := s.RegisterMessage(s.Request)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message:")
	}

	return s
}

//NewProtocol creates a protocol for stat, as you can see it is simultanously absolutely useless and regrettably necessary
func (st *Stat) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}
