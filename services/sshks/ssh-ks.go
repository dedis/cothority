// Package sshks offers functions to interact with the ssh-files. It depends
// on the golang/crypto/ssh library.
package sshks

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"sync"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	scosi "github.com/dedis/cothority/services/cosi"
	"github.com/dedis/crypto/abstract"
)

func init() {
	network.RegisterMessageType(ServerKS{})
	network.RegisterMessageType(ClientKS{})
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
	// Clients is a map of strings of public keys pointing to Clients
	Clients map[string]*Client
	// Signers denote the clients that signed
	Signers []*network.Entity
	// Signature by CoSi
	Signature *scosi.SignResponse
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
	}
	_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		return nil, err
	}
	c := msg.(Config)
	conf = &c
	if len(conf.Clients) == 0 {
		conf.Clients = make(map[string]*Client)
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
	for _, cl := range conf.Signers {
		dbg.Print("Signer", cl.Public)
		agg.Add(agg, cl.Public)
	}
	sig := conf.Signature

	if !bytes.Equal(conf.Hash(), sig.Sum) {
		return errors.New("Hash is different")
	}
	dbg.Print("Response is", sig.Response)
	if err := cosi.VerifySignature(network.Suite, sig.Sum, agg, sig.Challenge, sig.Response); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}

// EntityList makes a list of all servers with ourselves as the root
func (conf *Config) EntityList(root *network.Entity) *sda.EntityList {
	// The list is of length 1 with a capacity for all servers
	list := make([]*network.Entity, 1, len(conf.Servers))
	for _, srv := range conf.Servers {
		if srv.Entity.Addresses[0] == root.Addresses[0] {
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
	cop.Signers = nil
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
	dbg.Lvl3("Adding client", c, "to", conf.Clients, "key", c.SSHpub)
	conf.Clients[c.Entity.Public.String()] = c
	conf.Signature = nil
	return nil
}

// DelClient removes a client from the configuration-list
func (conf *Config) DelClient(c *Client) error {
	delete(conf.Clients, c.Entity.Public.String())
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

// List prints the version and a list of Servers and Clients
func (conf *Config) List() {
	dbg.Print("Config-version:", conf.Version)
	for _, srv := range conf.Servers {
		dbg.Print("Server:", srv.Entity.String())
	}
	for n, cl := range conf.Clients {
		dbg.Print("Client:", n, cl.Entity.String())
	}
}

// Sign uses the clients private key to sign off the config and returns
// the CoSi-structure and the commitment
func (conf *Config) Sign(cl *ClientKS, comm *cosi.Commitment) (*cosi.Cosi, *cosi.Commitment) {
	//c := cosi.NewCosi(network.Suite, cl.Private)
	return nil, nil
}

// Copy makes a deep-copy of the config by marshalling and then unmarshalling it
func (conf *Config) Copy() (*Config, error) {
	b, err := network.MarshalRegisteredType(conf)
	if err != nil {
		return nil, err
	}
	_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		return nil, err
	}
	newConf := msg.(Config)
	if len(newConf.Clients) == 0 {
		newConf.Clients = make(map[string]*Client)
	}
	if len(newConf.Servers) == 0 {
		newConf.Servers = make(map[string]*Server)
	}
	return &newConf, nil

}

// SetupTmpHosts sets up a temporary .tmp-directory for testing
func SetupTmpHosts() (string, error) {
	tmp, err := ioutil.TempDir("", "testHost")
	if err != nil {
		return "", errors.New("Coulnd't create tmp-dir: " + err.Error())
	}
	err = CreateBogusSSH(tmp, "id_rsa")
	if err != nil {
		return "", err
	}
	err = CreateBogusSSH(tmp, "ssh_host_rsa_key")
	if err != nil {
		return "", err
	}

	return tmp, nil
}

type sshKey struct {
	priv []byte
	pub  []byte
}

// bKeys are bogus ssh-keys precomputed for faster testing
var bKeys []sshKey
var bKeysI int

// CreateBogusSSH creates a private/public key
func CreateBogusSSH(dir, file string) error {
	if bKeys == nil {
		// Pre-calculate some ssh-keys for faster testing
		tmp, err := ioutil.TempDir("", "makeSSH")
		if err != nil {
			return err
		}
		bKeys = make([]sshKey, 5)
		for i := range bKeys {
			file := tmp + "/ssh" + strconv.Itoa(i)
			out, err := exec.Command("ssh-keygen", "-t", "rsa", "-b", "4096", "-N", "", "-f",
				file).CombinedOutput()
			dbg.Lvl5(string(out))
			if err != nil {
				return err
			}
			priv, err := ioutil.ReadFile(file)
			if err != nil {
				return err
			}
			pub, err := ioutil.ReadFile(file + ".pub")
			if err != nil {
				return err
			}
			bKeys[i] = sshKey{priv, pub}
		}
	}
	dbg.Lvl4("Directory is:", dir)
	sk := bKeys[bKeysI]
	err := ioutil.WriteFile(dir+"/"+file, sk.priv, 0660)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(dir+"/"+file+".pub", sk.pub, 0660)
	if err != nil {
		return err
	}
	bKeysI = (bKeysI + 1) % len(bKeys)
	return nil
}
