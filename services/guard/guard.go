package guard

import (
	"crypto/rand"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// This file contains all the code to run a Guard service. The Guard receives takes a
// request for the Guard reports of the server, and sends back the message recieved encrypted with the key for each service
// in the server.

// ServiceName is the name to refer to the Guard service.
const ServiceName = "Guard"

var z []byte

func init() {
	sda.RegisterNewService(ServiceName, newGuardService)
	network.RegisterMessageType(&Request{})
	network.RegisterMessageType(&Response{})
	//This is the area where Z is generated for a server
	const n = 5
	lel := make([]byte, n)
	rand.Read(lel)
	z = []byte{1, 2, 3, 4, 5}

}

//This is the area where Z is generated for a server, it creates z, which is a bytestring of length n for each guard.

// Stat is the service that returns the Guard reports of all services running on a server.
type Guard struct {
	*sda.ServiceProcessor
	path string
}

// Request is what the Guard service is expected to receive from clients.
type Request struct {
	UID   []byte
	Epoch []byte
	Msg   []byte
}

// Response is what the Guard service will reply to clients.
type Response struct {
	Msg []byte
}

// Request treats external request to this service.
func (st *Guard) Request(e *network.ServerIdentity, req *Request) (network.Body, error) {
	la, _ := e.Public.Data()
	return &Response{abstract.Sum(network.Suite, req.Msg, z, la, req.UID, req.Epoch)}, nil
}

// newStatService creates a new service that is built for Guard
func newGuardService(c *sda.Context, path string) sda.Service {
	s := &Guard{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}
	err := s.RegisterMessage(s.Request)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message:")
	}

	return s
}

// NewProtocol creates a protocol for stat, as you can see it is simultanously absolutely useless and regrettably necessary.
func (st *Guard) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}
