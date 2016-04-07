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

// GetServer returns the Server of that host
type GetServer struct{}

// GetServerRet returns the server
type GetServerRet struct {
	Server *Server
}

// GetConfig returns the latest config-structure of the server
type GetConfig struct{}

// GetConfigRet returns the config
type GetConfigRet struct {
	Config *Config
}

// AddServer asks for that server to be added to the config
type AddServer struct {
	Server *Server
}

// DelServer asks for that server to be deleted from the config
type DelServer struct {
	Server *Server
}

// AddClient asks for addition of a client
type AddClient struct {
	Client *Client
}

// DelClient asks for the removal of a client
type DelClient struct {
	Client *Client
}

// Sign asks all servers to sign on the config, will return the new config
type Sign struct{}

// SignRet is the new signed config
type SignRet struct {
	Config *Config
}

// StatusRet returns the success (empty string) or failure
type StatusRet struct {
	Error string
}

// FuncRegisterRet registers all messages to the network - not
// really necessary for the outgoing messages, but useful for
// external users
func FuncRegister() {
	var structs = []interface{}{
		GetServer{},
		GetServerRet{},
		GetConfig{},
		GetConfigRet{},
		AddServer{},
		DelServer{},
		AddClient{},
		DelClient{},
		Sign{},
		SignRet{},
		StatusRet{},
	}
	for _, s := range structs {
		network.RegisterMessageType(s)
	}
}

// NetworkGetServer asks for the Server at a given address
func NetworkGetServer(addr string) (*Server, error) {
	resp, err := networkSendAnonymous(addr, &GetServer{})
	if err != nil {
		return nil, err
	}
	conf, ok := resp.Msg.(GetServerRet)
	if !ok {
		return nil, errors.New("Didn't get Config back")
	}
	return conf.Server, nil
}

func networkSendAnonymous(addr string, req network.ProtocolMessage) (*network.NetworkMessage, error) {
	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)
	dst := network.NewEntity(kp.Public, addr)
	return networkSend(NewClientApp(""), dst, req)
}

func networkSend(src *ClientApp, dst *network.Entity, req network.ProtocolMessage) (*network.NetworkMessage, error) {
	client := network.NewSecureTcpHost(src.Private, nil)

	// Connect to the root
	dbg.Lvl3("Opening connection")
	con, err := client.Open(dst)
	defer client.Close()
	if err != nil {
		return &network.NetworkMessage{}, err
	}

	dbg.Lvl3("Sending sign request")
	pchan := make(chan network.NetworkMessage)
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
		pchan <- packet
	}()
	select {
	case response := <-pchan:
		dbg.Lvl5("Response:", response)
		return &response, nil
	case <-time.After(time.Second * 10):
		return &network.NetworkMessage{}, errors.New("Timeout on signing")
	}
}
