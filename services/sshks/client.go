package sshks

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"io/ioutil"
	"os"
)

// ClientKS represents one client and holds all necessary structures
type ClientKS struct {
	// This points to the client holding this structure
	This *Client
	// Config holds all servers and clients
	Config *Config
	// Private is our private key
	Private abstract.Secret
}

// NewClientKS creates a new private/public key pair and returns
// a ClientKS with an empty Config. It takes a public ssh-key.
func NewClientKS(sshPub string) *ClientKS {
	pair := config.NewKeyPair(network.Suite)
	return &ClientKS{NewClient(pair.Public, sshPub), nil, pair.Secret}
}

// ReadClientKS searches for the client-ks and creates a new one if it
// doesn't exist
func ReadClientKS(f string) (*ClientKS, error) {
	file := ExpandHDir(f)
	ca := NewClientKS("TestClient-" + f)
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		ca.Config = &Config{}
		return ca, nil
	}
	b, err := ioutil.ReadFile(ExpandHDir(file))
	if err != nil {
		return nil, err
	}
	_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		return nil, err
	}
	c, ok := msg.(ClientKS)
	if !ok {
		return nil, errors.New("Didn't get a ClientKS structure")
	}
	ca = &c
	return ca, nil
}

// Write takes a file and writes the clientKS to that file
func (ca *ClientKS) Write(file string) error {
	b, err := network.MarshalRegisteredType(ca)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(ExpandHDir(file), b, 0660)
	return err
}

// NetworkAddServer adds a new server to the Config
func (ca *ClientKS) NetworkAddServer(s *Server) error {
	if ca.Config == nil {
		return errors.New("No config available yet")
	}
	dbg.Lvl3("Servers are", ca.Config.Servers)
	for _, srv := range ca.Config.Servers {
		// Add the new server to all servers
		resp, err := NetworkSendAnonymous(srv.Entity.Addresses[0],
			&AddServer{s})
		err = ErrMsg(resp, err)
		if err != nil {
			return err
		}

		// Add the other servers to the new server
		resp, err = NetworkSendAnonymous(s.Entity.Addresses[0],
			&AddServer{srv})
		err = ErrMsg(resp, err)
		if err != nil {
			return err
		}
	}
	return nil
}

// NetworkDelServer deletes a server from the Config
func (ca *ClientKS) NetworkDelServer(s *Server) error {
	if ca.Config == nil {
		return errors.New("No config available yet")
	}
	for _, srv := range ca.Config.Servers {
		if srv.Entity.Addresses[0] == s.Entity.Addresses[0] {
			continue
		}
		dbg.Lvl3("Asking server", srv, "to delete server", s)
		resp, err := NetworkSendAnonymous(srv.Entity.Addresses[0],
			&DelServer{s})
		err = ErrMsg(resp, err)
		if err != nil {
			return err
		}
	}
	return nil
}

// NetworkAddClient adds a client to the configuration
func (ca *ClientKS) NetworkAddClient(c *Client) error {
	if ca.Config == nil {
		return errors.New("No config available yet")
	}
	dbg.Lvl3("Adding clients to", ca.Config.Servers)
	for _, srv := range ca.Config.Servers {
		dbg.Lvl3("Asking server", srv, "to add client", c)
		resp, err := NetworkSend(ca.Private, srv.Entity, &AddClient{c})
		err = ErrMsg(resp, err)
		if err != nil {
			return err
		}
	}
	return nil
}

// NetworkDelClient deletes a client from the configuration
func (ca *ClientKS) NetworkDelClient(c *Client) error {
	if ca.Config == nil {
		return errors.New("No config available yet")
	}
	for _, srv := range ca.Config.Servers {
		dbg.Lvl3("Asking server", srv, "to del client", c)
		resp, err := NetworkSend(ca.Private, srv.Entity, &DelClient{c})
		err = ErrMsg(resp, err)
		if err != nil {
			return err
		}
	}
	return nil
}

// NetworkGetConfig asks the server for its configuration
func (ca *ClientKS) NetworkGetConfig(s *Server) (*Config, error) {
	resp, err := NetworkSend(ca.Private, s.Entity, &GetConfig{})
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
func (ca *ClientKS) NetworkSign(s *Server) (*Config, error) {
	dbg.Lvl3("Asking server", s, "to sign")
	resp, err := NetworkSend(ca.Private, s.Entity, &Sign{})
	if err != nil {
		return nil, err
	}
	status, ok := resp.Msg.(StatusRet)
	if !ok {
		return nil, errors.New("Didn't receive config")
	}
	if status.Error != "" {
		dbg.Error(status.Error)
		return nil, errors.New(status.Error)
	}
	conf, err := ca.NetworkGetConfig(s)
	dbg.Lvl3("Got configuration", conf, err, "from", s)
	return conf, err
}

// ServerAdd adds a new server and asks all servers, including the new one,
// to sign off the new configuration
func (ca *ClientKS) ServerAdd(srvAddr string) error {
	srv, err := NetworkGetServer(srvAddr)
	if err != nil {
		return err
	}
	if len(ca.Config.Servers) > 0 {
		// Only add additional servers, because if it's the first server
		// we add, just sign and update the configuration
		err = ca.NetworkAddServer(srv)
		if err != nil {
			return err
		}
	} else {
		err = ca.Update(srv)
		if err != nil {
			return err
		}
	}
	return ca.Sign()
}

// ServerDel deletes a server and asks the remaining servers (if any)
// to sign the new configuration
func (ca *ClientKS) ServerDel(srvAddr string) error {
	srv, err := NetworkGetServer(srvAddr)
	if err != nil {
		return err
	}
	err = ca.NetworkDelServer(srv)
	if err != nil {
		return err
	}
	if len(ca.Config.Servers) == 1 {
		dbg.Lvl2("Deleted last server")
		ca.Config = NewConfig(0)
	} else {
		delete(ca.Config.Servers, srv.Entity.Addresses[0])
		err := ca.Sign()
		if err != nil {
			return err
		}
	}
	dbg.Lvl3(ca.Config.Servers)
	return nil
}

// ServerCheck contacts all servers and verifies that all
// configurations are the same
func (ca *ClientKS) ServerCheck() error {
	if ca.Config == nil {
		return errors.New("No config defined")
	}
	if len(ca.Config.Servers) == 0 {
		return errors.New("No servers defined")
	}
	var cnf *Config
	for _, srv := range ca.Config.Servers {
		cnfSrv, err := ca.NetworkGetConfig(srv)
		dbg.ErrFatal(err)
		if cnf != nil {
			if bytes.Compare(cnf.Hash(), cnfSrv.Hash()) != 0 {
				return errors.New("Hashes should be the same\n" +
					"1st server has " + fmt.Sprintln(cnfSrv) +
					"Server" + fmt.Sprint(srv) + "has" + fmt.Sprintln(cnfSrv))
			}
			cnf = cnfSrv
		}
	}
	return nil
}

// ClientAdd adds a new client and signs the new configuration
func (ca *ClientKS) ClientAdd(client *Client) error {
	if len(ca.Config.Servers) == 0 {
		return errors.New("Missing servers. Please add one or more servers first")
	}
	err := ca.NetworkAddClient(client)
	if err != nil {
		return err
	}
	return ca.Sign()
}

// ClientDel deletes the client and signs the new configuration
func (ca *ClientKS) ClientDel(client *Client) error {
	if len(ca.Config.Servers) == 0 {
		return errors.New("Missing servers. Please add one or more servers first")
	}
	err := ca.NetworkDelClient(client)
	if err != nil {
		return err
	}
	return ca.Sign()
}

// Update checks for the latest configuration
// TODO: include SkipChains to get the latest cothority
func (ca *ClientKS) Update(srv *Server) error {
	conf := NewConfig(-1)
	if srv == nil {
		// If no server is given, we contact all servers and ask
		// for the latest version
		dbg.Lvl3("Going to search all servers")
		for _, s := range ca.Config.Servers {
			c, err := ca.NetworkGetConfig(s)
			if err == nil {
				if conf.Version < c.Version {
					conf = c
				}
			}
		}
	} else {
		// If a server is given, we use that one
		dbg.Lvl3("Using server", srv, "to update")
		var err error
		conf, err = ca.NetworkGetConfig(srv)
		if err != nil {
			return err
		}
	}
	ca.Config = conf
	return nil
}

// Sign checks for any server and asks it to start
// a signing round
func (ca *ClientKS) Sign() error {
	srv, err := ca.getAnyServer()
	if err != nil {
		return err
	}
	dbg.Lvl3("Asking server", srv.Entity.Addresses[0], "for signature")
	conf, err := ca.NetworkSign(srv)
	if err != nil {
		return err
	}
	ca.Config = conf
	return nil
}

// Gets any server from the config
func (ca *ClientKS) getAnyServer() (*Server, error) {
	for _, srv := range ca.Config.Servers {
		return srv, nil
	}
	return nil, errors.New("Found no servers")
}
