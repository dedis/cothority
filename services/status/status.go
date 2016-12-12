package status

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

// This file contains all the code to run a Stat service. The Stat receives takes a
// request for the Status reports of the server, and sends back the status reports for each service
// in the server.

// ServiceName is the name to refer to the Status service.
const ServiceName = "Status"

func init() {
	sda.RegisterNewService(ServiceName, newStatService)
	network.RegisterPacketType(&Request{})
	network.RegisterPacketType(&Response{})

}

// Stat is the service that returns the status reports of all services running on a server.
type Stat struct {
	*sda.ServiceProcessor
	path string
}

// Request is what the Status service is expected to receive from clients.
type Request struct{}

type Status struct {
	Field map[string]string
}

// Response is what the Status service will reply to clients.
type Response struct {
	Msg            map[string]*Status
	ServerIdentity *network.ServerIdentity
}

// Request treats external request to this service.
func (st *Stat) Request(req *Request) (network.Body, sda.ClientError) {
	log.Lvl3("Returning", st.Context.ReportStatus())
	ret := &Response{
		Msg:            make(map[string]*Status),
		ServerIdentity: st.ServerIdentity(),
	}
	for k, v := range st.Context.ReportStatus() {
		ret.Msg[k] = &Status{Field: make(map[string]string)}
		for fk, fv := range v {
			ret.Msg[k].Field[fk] = fv
		}
	}
	return ret, nil
}

// newStatService creates a new service that is built for Status
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
