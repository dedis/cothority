package guard

import (
	"crypto/aes"
	"crypto/cipher"
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
	network.RegisterMessageType(&Request{})
	network.RegisterMessageType(&Response{})
}

//This is the area where Z is generated for a server, it creates z, which is a bytestring of length n for each guard.

// Guard is a structure that stores the guards secret key, z, to be used later in the process of hashing the clients requests
type Guard struct {
	*sda.ServiceProcessor
	path string
	z    []byte
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
	//hashy computes the hash that should be sent back to the main server H(pwhash, x, UID, Epoch)
	hashy := abstract.Sum(network.Suite, req.Msg, st.z, req.UID, req.Epoch)
	key := []byte("example key 1234")
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	// This part is necessary because you need a key to convert the Hash to a stream. The key is not even important because
	//all that is required is that the request gives the same output for the same input
	stream := cipher.NewCTR(block, []byte("example key 1234"))
	msg := make([]byte, 88)
	//this does not work, as it merely appends zeros at the end, this is still necessary to do an xor during the clients
	//password setting and authentication services
	stream.XORKeyStream(msg, hashy)

	return &Response{msg}, nil
}

// newGuardService creates a new service that is built for Guard
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
	rand.Read(lel)
	s.z = lel

	return s
}

// NewProtocol creates a protocol for stat, as you can see it is simultanously absolutely useless and regrettably necessary.
func (st *Guard) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	return nil, nil
}
