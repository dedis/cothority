package status

import (
	"sort"

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

// Stat is the service that handles collective signing operations
type Stat struct {
	*sda.ServiceProcessor
	path string
}

// Request is what the Cosi service is expected to receive from clients.
type Request struct{}

// Response is what the Cosi service will reply to clients.
type Response struct {
	Serv      string
	Tot       int
	Remote    []string
	Received  []uint64
	Sent      []uint64
	Available []string
}

//Received gives packets received
func Received(n map[network.ServerIdentityID]network.SecureConn) []uint64 {
	var a []uint64
	for _, value := range n {
		a = append(a, value.Rx())
	}
	return a
}

//Sent gives packets sent
func Sent(n map[network.ServerIdentityID]network.SecureConn) []uint64 {
	var a []uint64
	for _, value := range n {
		a = append(a, value.Tx())
	}
	return a

}

//Host is the host
func Host(n map[network.ServerIdentityID]network.SecureConn) string {
	var a string
	for _, value := range n {
		a = value.Local()
	}
	return a

}

//Remote is who the host is connected to
func Remote(n map[network.ServerIdentityID]network.SecureConn) []string {
	var a []string
	for _, value := range n {
		a = append(a, value.Remote())
	}
	return a
}

// Request treats external request to this service.
func (st *Stat) Request(e *network.ServerIdentity, req *Request) (network.Body, error) {
	sorted := sda.Available()
	sort.Strings(sorted)
	return &Response{
		Serv:      Host(st.Context.Host.Connections),
		Remote:    Remote(st.Context.Host.Connections),
		Tot:       len(st.Context.Host.Connections),
		Received:  Received(st.Context.Host.Connections),
		Sent:      Sent(st.Context.Host.Connections),
		Available: sorted,
	}, nil
}

func newStatService(c sda.Context, path string) sda.Service {
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
