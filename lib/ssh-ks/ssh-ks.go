// Package ssh_ks offers functions to interact with the ssh-files. It depends
// on the golang/crypto/ssh library.
package ssh_ks

import (
	"bytes"
	"errors"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	_ "github.com/dedis/cothority/protocols/cosi"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"io/ioutil"
	"strings"
)

type Server struct {
	// Entity is our identity - IP and public key
	Entity *network.Entity
	// Private key of that server
	Private abstract.Secret
	// Config is the configuration that is known actually to the server
	Config *Config
	// host represents the actual running host
	host *sda.Host
}

func NewServer(key *config.KeyPair, addr string) *Server {
	e := network.NewEntity(key.Public, addr)
	s := &Server{
		Entity:  e,
		Private: key.Secret,
		Config:  NewConfig(0),
	}
	s.AddServer(s)
	return s
}

// AddServer inserts a server in the configuration-list
func (s *Server) AddServer(s2 *Server) error {
	s.Config.Servers[s2.Entity.Addresses[0]] = s2.Entity
	s.Config.Signature = nil
	return nil
}

// DelServer removes a server from the configuration-list
func (s *Server) DelServer(s2 *Server) error {
	delete(s.Config.Servers, s2.Entity.Addresses[0])
	s.Config.Signature = nil
	return nil
}

// Start opens the port indicated for listening
func (s *Server) Start() error {
	s.host = sda.NewHost(s.Entity, s.Private)
	s.host.Listen()
	s.host.StartProcessMessages()
	return nil
}

// Stop closes the connection
func (s *Server) Stop() error {
	if s.host != nil {
		err := s.host.Close()
		if err != nil {
			return err
		}
		s.host.WaitForClose()
		s.host = nil
	}
	return nil
}

// Check searches for all servers and tries to connect
func (s *Server) Check() error {
	for _, s := range s.Config.Servers {
		list := sda.NewEntityList([]*network.Entity{s})
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

func (s *Server) Sign() error {
	s.Config.Version += 1
	s.Config.Signature = nil
	msg := s.Config.Hash()
	var err error
	s.Config.Signature, err = cosi.SignStatement(bytes.NewReader(msg),
		s.Config.EntityList(s.Entity))
	if err != nil {
		return err
	}
	return nil
}

type Config struct {
	// Version holds an incremental number of that version
	Version int
	// Servers is a map of IP:Port pointing to the network-entities
	Servers map[string]*network.Entity
	// PublicServers holds the public ssh-keys of all servers, mapped by their IP:Port
	PublicServers map[string]string
	// Clients is a map of IP:Port pointing to the network-entities
	Clients map[string]*network.Entity
	// PublicClients holds the public ssh-keys of all clients, mapped by their name
	PublicClients map[string]string
	// Signature by CoSi
	Signature *sda.CosiResponse
}

// NewConfig returns a new initialized config for the configuration-chain
func NewConfig(v int) *Config {
	return &Config{
		Version:       v,
		Servers:       make(map[string]*network.Entity),
		PublicServers: make(map[string]string),
		PublicClients: make(map[string]string),
	}
}

// VerifySignature takes an aggregate public key and checks it against the
// signature
func (c *Config) VerifySignature(agg abstract.Point) error {
	sig := c.Signature
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), c.Hash())
	hashHash, _ := crypto.HashBytes(network.Suite.Hash(), fHash)
	if !bytes.Equal(hashHash, sig.Sum) {
		return errors.New("Hash is different")
	}
	if err := cosi.VerifySignature(network.Suite, fHash, agg, sig.Challenge, sig.Response); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}

// EntityList makes a list of all servers with ourselves as the root
func (c *Config) EntityList(root *network.Entity) *sda.EntityList {
	// The list is of length 1 with a capacity for all servers
	list := make([]*network.Entity, 1, len(c.Servers))
	for _, ent := range c.Servers {
		if ent == root {
			list[0] = ent
		} else {
			list = append(list, ent)
		}
	}
	return sda.NewEntityList(list)
}

// Hash returns the hash of everything but the signature
func (c *Config) Hash() []byte {
	cop := *c
	cop.Signature = nil
	hash, _ := crypto.HashArgs(network.Suite.Hash(), cop)
	return hash
}

// setupTmpHosts sets up a temporary .tmp-directory for testing
func SetupTmpHosts() (string, error) {
	tmp, err := ioutil.TempDir("", "testHost")
	if err != nil {
		return "", errors.New("Coulnd't create tmp-dir: " + err.Error())
	}
	return tmp, nil
}
