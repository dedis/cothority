// Package status is a service for reporting the all the services running on a
// server.
//
// This file contains all the code to run a Stat service. The Stat receives
// takes a request for the Status reports of the server, and sends back the
// status reports for each service in the server.
package status

import (
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"
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

// Request treats external request to this service.
func (st *Stat) Request(req *Request) (network.Message, error) {
	statuses := st.Context.ReportStatus()
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
