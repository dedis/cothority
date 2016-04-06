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
	var structs = []struct {
		S interface{}
	}{
		{GetServer{}},
		{GetServerRet{}},
		{GetConfig{}},
		{GetConfigRet{}},
		{AddServer{}},
		{DelServer{}},
		{Sign{}},
		{SignRet{}},
		{StatusRet{}},
	}
	for _, s := range structs {
		network.RegisterMessageType(s.S)
	}
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
		return &StatusRet{"Didn't get a server"}
	}
	dbg.Lvl3("Adding server", req.Server, "to", c.This)
	c.AddServer(req.Server)
	return &StatusRet{""}
}

// FuncDelServer removes a given server from the configuration
func (c *ServerApp) FuncDelServer(data *network.NetworkMessage) network.ProtocolMessage {
	req, ok := data.Msg.(DelServer)
	if !ok {
		return &StatusRet{"Didn't get a server"}
	}
	if c.This.Entity.Addresses[0] == req.Server.Entity.Addresses[0] {
		return &StatusRet{"Cannot delete own address"}
	}
	c.DelServer(req.Server)
	return &StatusRet{""}
}

// FuncSign asks all servers to sign the new configuration
func (c *ServerApp) FuncSign(data *network.NetworkMessage) network.ProtocolMessage {
	status := &StatusRet{}
	err := c.Sign()
	if err != nil {
		status = &StatusRet{err.Error()}
	}
	return status
}

// NetworkAddServer adds a new server to the Config
func (ca *ClientApp) NetworkAddServer(s *Server) error {
	if ca.Config == nil {
		return errors.New("No config available yet")
	}
	dbg.Lvl3("Servers are", ca.Config.Servers)
	for _, srv := range ca.Config.Servers {
		// Add the new server to all servers
		resp, err := networkSendAnonymous(srv.Entity.Addresses[0],
			&AddServer{s})
		if err != nil {
			return err
		}
		errMsg := resp.Msg.(StatusRet).Error
		if errMsg != "" {
			return errors.New("Remote-error: " + errMsg)
		}
		// Add the other servers to the new server
		resp, err = networkSendAnonymous(s.Entity.Addresses[0],
			&AddServer{srv})
		if err != nil {
			return err
		}
		errMsg = resp.Msg.(StatusRet).Error
		if errMsg != "" {
			return errors.New("Remote-error: " + errMsg)
		}
	}
	return nil
}

// NetworkDelServer deletes a server from the Config
func (ca *ClientApp) NetworkDelServer(s *Server) error {
	if ca.Config == nil {
		return errors.New("No config available yet")
	}
	for addr, srv := range ca.Config.Servers {
		if srv.Entity.Addresses[0] == s.Entity.Addresses[0] {
			delete(ca.Config.Servers, addr)
			continue
		}
		resp, err := networkSendAnonymous(srv.Entity.Addresses[0],
			&DelServer{s})
		if err != nil {
			return err
		}
		errMsg := resp.Msg.(StatusRet).Error
		if errMsg != "" {
			return errors.New("Remote-error: " + errMsg)
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

// NetworkSign asks the servers to sign the new configuration
func (ca *ClientApp) NetworkSign(s *Server) (*Config, error) {
	resp, err := networkSend(ca, s.Entity, &Sign{})
	if err != nil {
		return nil, err
	}
	status, ok := resp.Msg.(StatusRet)
	if !ok {
		return nil, errors.New("Didn't receive config")
	}
	if status.Error != "" {
		return nil, errors.New(status.Error)
	}
	return ca.NetworkGetConfig(s)
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
