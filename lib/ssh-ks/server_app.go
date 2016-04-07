package ssh_ks

import (
	"bytes"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"strings"
)

// CoNode is the representation of this Node of the Cothority
type ServerApp struct {
	// Ourselves is the identity of this node
	This *Server
	// Private key of ourselves
	Private abstract.Secret
	// Config is the configuration that is known actually to the server
	Config *Config
	// host represents the actual running host
	host *sda.Host
}

// NewCoNode creates a new node of the cothority and initializes the
// Config-structures. It doesn't start the node
func NewServerApp(key *config.KeyPair, addr, sshdPub string) *ServerApp {
	srv := NewServer(key.Public, addr, sshdPub)
	c := &ServerApp{
		This:    srv,
		Private: key.Secret,
		Config:  NewConfig(0),
	}
	c.AddServer(srv)
	return c
}

// AddServer inserts a server in the configuration-list
func (c *ServerApp) AddServer(s *Server) error {
	c.Config.AddServer(s)
	return nil
}

// DelServer removes a server from the configuration-list
func (c *ServerApp) DelServer(s *Server) error {
	c.Config.DelServer(s)
	return nil
}

// Start opens the port indicated for listening
func (c *ServerApp) Start() error {
	c.host = sda.NewHost(c.This.Entity, c.Private)
	c.host.RegisterMessage(GetServer{}, c.FuncGetServer)
	c.host.RegisterMessage(GetConfig{}, c.FuncGetConfig)
	c.host.RegisterMessage(AddServer{}, c.FuncAddServer)
	c.host.RegisterMessage(DelServer{}, c.FuncDelServer)
	c.host.RegisterMessage(AddClient{}, c.FuncAddClient)
	c.host.RegisterMessage(DelClient{}, c.FuncDelClient)
	c.host.RegisterMessage(Sign{}, c.FuncSign)
	c.host.Listen()
	c.host.StartProcessMessages()
	return nil
}

// Stop closes the connection
func (c *ServerApp) Stop() error {
	if c.host != nil {
		err := c.host.Close()
		if err != nil {
			return err
		}
		c.host.WaitForClose()
		c.host = nil
	}
	return nil
}

// Check searches for all CoNodes and tries to connect
func (c *ServerApp) Check() error {
	for _, s := range c.Config.Servers {
		list := sda.NewEntityList([]*network.Entity{s.Entity})
		msg := "ssh-ks test"
		sig, err := cosi.SignStatement(strings.NewReader(msg), list)
		if err != nil {
			return err
		} else {
			err := cosi.VerifySignatureHash([]byte(msg), sig, list)
			if err != nil {
				return err
			}
			dbg.Lvl3("Received signature successfully")
		}
	}
	return nil
}

// Sign sends updates the configuration-structure by increasing the
// version and asks the cothority to sign the new structure.
func (c *ServerApp) Sign() error {
	c.Config.Version += 1
	c.Config.Signature = nil
	msg := c.Config.Hash()
	msg2 := c.Config.Hash()
	if bytes.Compare(msg, msg2) != 0 {
		dbg.Fatal("Hashing is different")
	}
	var err error
	c.Config.Signature, err = cosi.SignStatement(bytes.NewReader(msg),
		c.Config.EntityList(c.This.Entity))
	if err != nil {
		return err
	}
	return nil
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

// FuncAddClient adds a given client to the configuration
func (c *ServerApp) FuncAddClient(data *network.NetworkMessage) network.ProtocolMessage {
	req, ok := data.Msg.(AddClient)
	if !ok {
		return &StatusRet{"Didn't get a client"}
	}
	dbg.Lvl3("Adding client", req.Client, "to", c.This)
	c.Config.AddClient(req.Client)
	return &StatusRet{""}
}

// FuncDelServer removes a given server from the configuration
func (c *ServerApp) FuncDelClient(data *network.NetworkMessage) network.ProtocolMessage {
	req, ok := data.Msg.(DelClient)
	if !ok {
		return &StatusRet{"Didn't get a client"}
	}
	if c.This.Entity.Addresses[0] == req.Client.SSHpub {
		return &StatusRet{"Cannot delete own address"}
	}
	c.Config.DelClient(req.Client)
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
