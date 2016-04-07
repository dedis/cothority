// Package ssh_ks offers functions to interact with the ssh-files. It depends
// on the golang/crypto/ssh library.
package ssh_ks

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	_ "github.com/dedis/cothority/protocols/cosi"
	"github.com/dedis/crypto/abstract"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"sync"
)

func init() {
	network.RegisterMessageType(ServerApp{})
	network.RegisterMessageType(ClientApp{})
	network.RegisterMessageType(Config{})
	network.RegisterMessageType(Server{})
	network.RegisterMessageType(Client{})
}

// Server represents one server of the cothority
type Server struct {
	// Entity is the address-public key of that server
	Entity *network.Entity
	// SSHDpub is the public key of that ssh-daemon
	SSHDpub string
}

// NewServer creates a pointer to a new server
func NewServer(pub abstract.Point, addr, sshdPub string) *Server {
	return &Server{
		Entity:  network.NewEntity(pub, addr),
		SSHDpub: sshdPub,
	}
}

// Client represents one client that can access the cothority
type Client struct {
	// Public key of the client - stored as Entity for later
	// participation of clients in the Cothority
	Entity *network.Entity
	// SSHpub is the public key of its ssh-identity
	SSHpub string
}

// NewClient creates a new client given a public key and a public
// ssh-key
func NewClient(public abstract.Point, sshPub string) *Client {
	return &Client{network.NewEntity(public, ""), sshPub}
}

// Config holds everything that needs to be signed by the cothority
// and transferred to the clients
type Config struct {
	// Version holds an incremental number of that version
	Version int
	// Servers is a map of IP:Port pointing to Servers
	Servers map[string]*Server
	// Clients is a map of IP:Port pointing to Clients
	Clients map[string]*Client
	// Signature by CoSi
	Signature *sda.CosiResponse
}

// NewConfig returns a new initialized config for the configuration-chain
func NewConfig(v int) *Config {
	return &Config{
		Version: v,
		Servers: make(map[string]*Server),
		Clients: make(map[string]*Client),
	}
}

// ReadConfig searches for the config-file and creates a new one if it
// doesn't exist
func ReadConfig(file string) (*Config, error) {
	conf := &Config{}
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return conf, nil
	}
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	} else {
		_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
		if err != nil {
			return nil, err
		}
		c := msg.(Config)
		conf = &c
	}
	return conf, nil
}

// WriteConfig takes a file and writes the configuration to that file
func (conf *Config) WriteConfig(file string) error {
	b, err := network.MarshalRegisteredType(conf)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(file, b, 0660)
	return err
}

// VerifySignature calculates the aggregate public signature from all
// servers and verifies the signature against it.
func (conf *Config) VerifySignature() error {
	// Calculate aggregate public key
	agg := network.Suite.Point().Null()
	for _, srv := range conf.Servers {
		agg.Add(agg, srv.Entity.Public)
	}
	sig := conf.Signature

	// Double-hash the Config.Hash(), as this is what the signature
	// does
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), conf.Hash())
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
func (conf *Config) EntityList(root *network.Entity) *sda.EntityList {
	// The list is of length 1 with a capacity for all servers
	list := make([]*network.Entity, 1, len(conf.Servers))
	for _, srv := range conf.Servers {
		if srv.Entity == root {
			list[0] = srv.Entity
		} else {
			list = append(list, srv.Entity)
		}
	}
	return sda.NewEntityList(list)
}

var hashLock sync.Mutex

// Hash returns the hash of everything but the signature
func (conf *Config) Hash() []byte {
	hashLock.Lock()
	cop := *conf
	cop.Signature = nil
	hash, err := crypto.HashArgs(sha256.New(), &cop)
	if err != nil {
		dbg.Fatal(err)
	}
	hashLock.Unlock()
	return hash
}

// AddServer inserts a server in the configuration-list
func (conf *Config) AddServer(s *Server) error {
	conf.Servers[s.Entity.Addresses[0]] = s
	conf.Signature = nil
	return nil
}

// DelServer removes a server from the configuration-list
func (conf *Config) DelServer(s *Server) error {
	delete(conf.Servers, s.Entity.Addresses[0])
	conf.Signature = nil
	return nil
}

// AddClient inserts a client in the configuration-list
func (conf *Config) AddClient(c *Client) error {
	conf.Clients[c.SSHpub] = c
	conf.Signature = nil
	return nil
}

// DelClient removes a client from the configuration-list
func (conf *Config) DelClient(c *Client) error {
	delete(conf.Clients, c.SSHpub)
	conf.Signature = nil
	return nil
}

// MarshalBinary takes care of all maps to give them back in correct
// order
func (conf *Config) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, uint32(conf.Version))
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(conf.Servers))
	for i := range conf.Servers {
		keys = append(keys, i)
	}
	sort.Strings(keys)
	for _, s := range keys {
		_, err = buf.WriteString(s)
		if err != nil {
			return nil, err
		}
		b, err := network.MarshalRegisteredType(conf.Servers[s].Entity)
		if err != nil {
			return nil, err
		}
		_, err = buf.Write(b)
		if err != nil {
			return nil, err
		}
	}
	keys = make([]string, 0, len(conf.Clients))
	for i := range conf.Clients {
		keys = append(keys, i)
	}
	sort.Strings(keys)
	for _, s := range keys {
		_, err = buf.WriteString(s)
		if err != nil {
			return nil, err
		}
		client := conf.Clients[s]
		_, err = buf.WriteString(client.SSHpub)
		if err != nil {
			return nil, err
		}
		b, err := network.MarshalRegisteredType(client.Entity)
		if err != nil {
			return nil, err
		}
		_, err = buf.Write(b)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// SetupTmpHosts sets up a temporary .tmp-directory for testing
func SetupTmpHosts() (string, error) {
	tmp, err := ioutil.TempDir("", "testHost")
	if err != nil {
		return "", errors.New("Coulnd't create tmp-dir: " + err.Error())
	}
	err = createBogusSSH(tmp, "id_rsa")
	if err != nil {
		return "", err
	}
	err = createBogusSSH(tmp, "ssh_host_rsa_key")
	if err != nil {
		return "", err
	}

	return tmp, nil
}

// createBogusSSH creates a private/public key
func createBogusSSH(dir, file string) error {
	dbg.Lvl2("Directory is:", dir)
	out, err := exec.Command("ssh-keygen", "-t", "rsa", "-b", "4096", "-N", "", "-f",
		dir+"/"+file).CombinedOutput()
	dbg.Lvl5(string(out))
	if err != nil {
		return err
	}
	return nil
}
