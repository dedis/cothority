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

// AddServerRet returns the success or failure
type AddServerRet struct {
	OK bool
}

// FuncRegisterRet registers all messages to the network - not
// really necessary for the outgoing messages, but useful for
// external users
func FuncRegister() {
	network.RegisterMessageType(GetServer{})
	network.RegisterMessageType(GetServerRet{})
	network.RegisterMessageType(GetConfig{})
	network.RegisterMessageType(GetConfigRet{})
	network.RegisterMessageType(AddServer{})
	network.RegisterMessageType(AddServerRet{})
}

// FuncGetServer returns our Server
func (c *ServerApp) FuncGetServer(*network.NetworkMessage) network.ProtocolMessage {
	return &GetServerRet{c.This}
}

// FuncGetConfig returns our Config
func (c *ServerApp) FuncGetConfig(*network.NetworkMessage) network.ProtocolMessage {
	return &GetConfigRet{c.Config}
}

// FuncAddServer adds a given server to the configuration
func (c *ServerApp) FuncAddServer(data *network.NetworkMessage) network.ProtocolMessage {
	req, ok := data.Msg.(AddServer)
	if !ok {
		return &AddServerRet{false}
	}
	c.AddServer(req.Server)
	return &AddServerRet{true}
}

// NetworkAddServer adds a new server to the Config
func (ca *ClientApp) NetworkAddServer(s *Server) error {
	if ca.Config == nil {

	} else {
		for _, srv := range ca.Config.Servers {
			dbg.Print("Asking for adding")
			resp, err := networkSendAnonymous(srv.Entity.Addresses[0],
				&AddServer{s})
			if err != nil {
				return err
			}
			if !resp.Msg.(AddServerRet).OK {
				return errors.New("Remote replied not OK")
			}
		}
	}
	return nil
}

// NetworkGetConfig asks the server for its configuration
func (ca *ClientApp) NetworkGetConfig(s *Server) (*Config, error) {
	resp, err := networkSend(ca, s.Entity, &GetConfig{})
	if err != nil {
		return nil, err
	}
	gcr, ok := resp.Msg.(GetConfigRet)
	if !ok {
		return nil, errors.New("Didn't receive config")
	}
	return gcr.Config, nil
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
