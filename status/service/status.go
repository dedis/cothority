// Package status is a service for reporting the all the services running on a
// server.
//
// This file contains all the code to run a Stat service. The Stat receives
// takes a request for the Status reports of the server, and sends back the
// status reports for each service in the server.
package status

import (
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

// ServiceName is the name to refer to the Status service.
const ServiceName = "Status"

func init() {
	onet.RegisterNewService(ServiceName, newStatService)
	network.RegisterMessage(&Request{})
	network.RegisterMessage(&Response{})
}

// Stat is the service that returns the status reports of all services running
// on a server.
type Stat struct {
	*onet.ServiceProcessor
}

// Version will be set by the main() function before starting the server.
var Version = "unknown"

// Request treats external request to this service.
func (st *Stat) Request(req *Request) (network.Message, error) {
	statuses := st.Context.ReportStatus()

	// Add this in here, because onet no longer knows the version, it is just a support
	// library and should never really have known it.
	statuses["Conode"] = &onet.Status{Field: make(map[string]string)}
	statuses["Conode"].Field["version"] = Version

	log.Lvl4("Returning", statuses)
	return &Response{
		Status:         statuses,
		ServerIdentity: st.ServerIdentity(),
	}, nil
}

// newStatService creates a new service that is built for Status
func newStatService(c *onet.Context) (onet.Service, error) {
	s := &Stat{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	err := s.RegisterHandler(s.Request)
	if err != nil {
		return nil, err
	}

	return s, nil
}
