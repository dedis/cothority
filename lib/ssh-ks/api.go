package ssh_ks

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
	"time"
)

// Here is the external API

func init() {
	FuncRegister()
}

// GetEntity returns the Entity of that host
type GetEntity struct{}

type GetEntityRet struct {
	Entity *network.Entity
}

type AddServer struct {
}

type AddServerRet struct {
	OK bool
}

// FuncRegisterRet registers all messages to the network - not
// really necessary for the outgoing messages, but useful for
// external users
func FuncRegister() {
	network.RegisterMessageType(GetEntity{})
	network.RegisterMessageType(GetEntityRet{})
	network.RegisterMessageType(AddServer{})
	network.RegisterMessageType(AddServerRet{})
}

func NetworkGetEntity(addr string) (*network.Entity, error) {
	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)

	// create a throw-away entity with an empty  address:
	client := network.NewSecureTcpHost(kp.Secret, nil)
	req := &GetEntity{}

	// Connect to the root
	host := network.NewEntity(kp.Public, addr)
	dbg.Lvl3("Opening connection")
	con, err := client.Open(host)
	defer client.Close()
	if err != nil {
		return nil, err
	}

	dbg.Lvl3("Sending sign request")
	pchan := make(chan GetEntityRet)
	go func() {
		// send the request
		if err := con.Send(context.TODO(), req); err != nil {
			close(pchan)
			return
		}
		dbg.Lvl3("Waiting for the response")
		// wait for the response
		packet, err := con.Receive(context.TODO())
		if err != nil {
			close(pchan)
			return
		}
		pchan <- packet.Msg.(GetEntityRet)
	}()
	select {
	case response, ok := <-pchan:
		dbg.Lvl5("Response:", ok, response)
		if !ok {
			return nil, errors.New("Invalid repsonse: Could not cast the " +
				"received response to the right type")
		}
		return response.Entity, nil
	case <-time.After(time.Second * 10):
		return nil, errors.New("Timeout on signing")
	}

}

// FuncGetEntity returns our Entity
func (c *CoNode) FuncGetEntity(*network.NetworkMessage) network.ProtocolMessage {
	return &GetEntityRet{c.Ourselves.Entity}
}

// FuncAddServer adds a given server to the configuration
func (c *CoNode) FuncAddServer(*network.NetworkMessage) network.ProtocolMessage {
	return &GetEntityRet{c.Ourselves.Entity}
}
