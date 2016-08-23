package guard

import (
	"crypto/rand"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// This file contains all the code to run a Guard service. The Guard receives takes a
// request for the Guard password protection service and sends back the message received encrypted with the key for each service
// in the server.

// ServiceName is the name to refer to the Guard service.
const ServiceName = "Guard"

func init() {
	sda.RegisterNewService(ServiceName, newGuardService)
	network.RegisterPacketType(&Request{})
	network.RegisterPacketType(&Response{})
}

//This is the area where Z is generated for a server, it creates z, which is a bytestring of length n for each guard.

// Guard is a structure that stores the guards secret key, z, to be used later in the process of hashing the clients requests.
type Guard struct {
	*sda.ServiceProcessor
	path string
	Z    []byte
}

// Request is what the Guard service is expected to receive from clients.
type Request struct {
	UID   []byte
	Epoch []byte
	Msg   abstract.Point
}

// Response is what the Guard service will reply to clients.
type Response struct {
	Msg abstract.Point
}

// Request treats external request to this service.
func (st *Guard) Request(e *network.ServerIdentity, req *Request) (network.Body, error) {
	//hashy computes the hash that should be sent back to the main server H(pwhash, x, UID, Epoch)
	blankpoint := network.Suite.Point()
	zbytes := network.Suite.Scalar()
	zbytes.SetBytes(st.Z)
	//need to change this impementation, the setbytes will not work

	sendy := blankpoint.Mul(req.Msg, zbytes)
	return &Response{sendy}, nil
}

// newGuardService creates a new service that is built for Guard.
func newGuardService(c *sda.Context, path string) sda.Service {
	s := &Guard{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}
	err := s.RegisterMessage(s.Request)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message:")
	}

	//This is the area where Z is generated for a server
	const n = 88
	lel := make([]byte, n)
	_, err = rand.Read(lel)
	log.ErrFatal(err)
	s.Z = lel

	return s
}

// NewProtocol creates a protocol for stat, as you can see it is simultanously absolutely useless and regrettably necessary.
func (st *Guard) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}
