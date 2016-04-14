package sshks

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	libcosi "github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/cosi"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// ServerKS is the server KeyStorage and represents this Node of the Cothority
type ServerKS struct {
	// Ourselves is the identity of this node
	This *Server
	// Private key of ourselves
	Private abstract.Secret
	// Config is the configuration that is known actually to the server
	Config *Config
	// NextConfig represents our next configuration
	NextConfig *NextConfig
	// DirSSHD is the directory where the server's public key is stored
	DirSSHD string
	// DirSSH is the directory where the authorized_keys will be written to
	DirSSH string
	// host represents the actual running host
	host *sda.Host
}

// NewServerKS creates a new node of the cothority and initializes the
// Config-structures. It doesn't start the node
func NewServerKS(key *config.KeyPair, addr, dirSSHD, dirSSH string) (*ServerKS, error) {
	sshdPub, err := ioutil.ReadFile(dirSSHD + "/ssh_host_rsa_key.pub")
	if err != nil {
		return nil, err
	}
	srv := NewServer(key.Public, addr, string(sshdPub))
	c := &ServerKS{
		This:    srv,
		Private: key.Secret,
		Config:  NewConfig(0),
		DirSSHD: dirSSHD,
		DirSSH:  dirSSH,
	}
	c.AddServer(srv)
	c.NextConfig = NewNextConfig(c)
	return c, nil
}

// ReadServerKS reads a configuration file and returns a ServerKS
func ReadServerKS(f string) (*ServerKS, error) {
	file := ExpandHDir(f)
	if file == "" {
		return nil, errors.New("Need a name for the configuration-file")
	}
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return nil, errors.New("Didn't find file " + file)
	}
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		return nil, err
	}
	sa := msg.(ServerKS)
	return &sa, err
}

// WriteConfig takes the whole config and writes it into a file. It can be
// read back with ReadServerKS
func (sa *ServerKS) WriteConfig(file string) error {
	b, err := network.MarshalRegisteredType(sa)
	if err != nil {
		return err
	}
	ioutil.WriteFile(ExpandHDir(file), b, 0660)
	return nil
}

// AddServer inserts a server in the configuration-list
func (sa *ServerKS) AddServer(s *Server) error {
	sa.Config.AddServer(s)
	return nil
}

// DelServer removes a server from the configuration-list
func (sa *ServerKS) DelServer(s *Server) error {
	sa.Config.DelServer(s)
	return nil
}

// Start opens the port indicated for listening
func (sa *ServerKS) Start() error {
	sa.host = sda.NewHost(sa.This.Entity, sa.Private)
	sa.host.RegisterExternalMessage(SendFirstCommit{}, sa.FuncSendFirstCommit)
	sa.host.RegisterExternalMessage(SendNewConfig{}, sa.FuncSendNewConfig)
	sa.host.RegisterExternalMessage(GetServer{}, sa.FuncGetServer)
	sa.host.RegisterExternalMessage(GetConfig{}, sa.FuncGetConfig)
	sa.host.RegisterExternalMessage(Response{}, sa.FuncResponse)
	sa.host.RegisterExternalMessage(PropSig{}, sa.FuncPropSig)
	cosi.AddCosiApp(sa.host)
	sa.host.Listen()
	sa.host.StartProcessMessages()
	return nil
}

// WaitForClose calls the host equivalent and will only return once the
// connection is closed
func (sa *ServerKS) WaitForClose() {
	sa.host.WaitForClose()
}

// Stop closes the connection
func (sa *ServerKS) Stop() error {
	if sa.host != nil {
		err := sa.host.Close()
		if err != nil {
			return err
		}
		sa.host.WaitForClose()
		sa.host = nil
	}
	return nil
}

// Check searches for all CoNodes and tries to connect
func (sa *ServerKS) Check() error {
	for _, s := range sa.Config.Servers {
		list := sda.NewEntityList([]*network.Entity{s.Entity})
		msg := "ssh-ks test"
		sig, err := cosi.SignStatement(strings.NewReader(msg), list)
		if err != nil {
			return err
		}
		err = cosi.VerifySignatureHash([]byte(msg), sig, list)
		if err != nil {
			return err
		}
		dbg.Lvl3("Received signature successfully")
	}
	return nil
}

// FuncGetServer returns our Server
func (sa *ServerKS) FuncGetServer(*network.Message) network.ProtocolMessage {
	return &GetServerRet{sa.This}
}

// FuncSendFirstConfig stores the new config before it is signed by other clients
func (sa *ServerKS) FuncSendFirstCommit(data *network.Message) network.ProtocolMessage {
	if sa.unknownClient(data.Entity) {
		return &StatusRet{"Refusing unknown client"}
	}
	comm, ok := data.Msg.(SendFirstCommit)
	if !ok {
		return &StatusRet{"Didn't get a commit"}
	}
	sa.NextConfig.AddCommit(data.Entity, comm.Commitment)
	return &StatusRet{""}
}

// FuncSendNewConfig stores the new config before it is signed by other clients
func (sa *ServerKS) FuncSendNewConfig(data *network.Message) network.ProtocolMessage {
	if sa.unknownClient(data.Entity) {
		return &StatusRet{"Refusing unknown client"}
	}
	conf, ok := data.Msg.(SendNewConfig)
	if !ok {
		return &StatusRet{"Didn't get a config"}
	}
	chal, err := sa.NextConfig.NewConfig(sa, conf.Config)
	if err != nil {
		return &SendNewConfigRet{}
	}
	return &SendNewConfigRet{chal}
}

// FuncGetConfig returns our Config
func (sa *ServerKS) FuncGetConfig(*network.Message) network.ProtocolMessage {
	var newconf *Config
	if sa.NextConfig.config.Version > sa.Config.Version {
		newconf = sa.NextConfig.config
		dbg.LLvl3("Adding new config", *newconf)
	}
	return &GetConfigRet{
		Config:    sa.Config,
		NewConfig: newconf,
	}
}

// FuncSign asks all servers to sign the new configuration
func (sa *ServerKS) FuncResponse(data *network.Message) network.ProtocolMessage {
	if sa.unknownClient(data.Entity) {
		return &StatusRet{"Refusing unknown client"}
	}
	response, ok := data.Msg.(Response)
	if !ok {
		return &ResponseRet{}
	}
	ok = sa.NextConfig.AddResponse(data.Entity, response.Response)
	if ok {
		dbg.LLvl3("Storing new config version", sa.NextConfig.config.Version)
		sa.Config = sa.NextConfig.config
	}
	sa.NextConfig.AddCommit(data.Entity, response.NextCommitment)
	if sa.NextConfig == nil {
		dbg.LLvl3("No nextconfig yet - just storing commitment")
	} else {
		dbg.LLvl3("Just one response. ok is", ok)
		if ok {
			err := sa.Config.VerifySignature()
			if err != nil {
				dbg.Error("Signature is wrong - sending anyway",
					err)
			}
			return &ResponseRet{
				ClientsTot:    sa.NextConfig.clients,
				ClientsSigned: sa.NextConfig.signers,
				Config:        sa.Config,
			}
		}
	}

	return &ResponseRet{sa.NextConfig.clients, sa.NextConfig.signers, nil}
}

// FuncPropSig checks the hash and if it matches updates the signature
func (sa *ServerKS) FuncPropSig(data *network.Message) network.ProtocolMessage {
	if sa.unknownClient(data.Entity) {
		return &StatusRet{"Refusing unknown client"}
	}
	ps, ok := data.Msg.(PropSig)
	if !ok {
		return &StatusRet{"Didn't get a signature"}
	}
	cnf := *sa.Config
	cnf.Version = ps.Version
	cnf.Signature = ps.Signature
	if bytes.Compare(ps.Hash, cnf.Hash()) == 0 {
		err := cnf.VerifySignature()
		if err != nil {
			return &StatusRet{"Signature doesn't match"}
		}
		sa.Config = &cnf
	} else {
		return &StatusRet{"Hashes don't match"}
	}
	return &StatusRet{""}
}

// CreateAuth takes all client public keys and writes them into a authorized_keys
// file
func (sa *ServerKS) CreateAuth() error {
	lines := make([]string, 0, len(sa.Config.Clients))
	for _, c := range sa.Config.Clients {
		lines = append(lines, c.SSHpub)
	}
	return ioutil.WriteFile(sa.DirSSH+"/authorized_keys",
		[]byte(strings.Join(lines, "\n")), 0600)
}

// unknownClient returns true if there are clients but non match
// the Entity given
func (sa *ServerKS) unknownClient(e *network.Entity) bool {
	if len(sa.Config.Clients) == 0 {
		// Accept any client if there are none
		return false
	}
	_, known := sa.Config.Clients[e.Public.String()]
	return !known
}

// ErrMsg converts a combined err and status-message to an error
func ErrMsg(status *network.Message, err error) error {
	if err != nil {
		return err
	}
	statusMsg, ok := status.Msg.(StatusRet)
	if !ok {
		return errors.New("Didn't get a StatusRet")
	}
	errMsg := statusMsg.Error
	if errMsg != "" {
		return errors.New("Remote-error: " + errMsg)
	}
	return nil
}

// NextConfig holds all things necessary to create a new configuration
type NextConfig struct {
	// Config is the next proposed configuration
	config *Config
	// Commits is a map of public-keys to pre-computed commits from the clients
	commits map[abstract.Point]*libcosi.Commitment
	// Responses holds all responses received so far
	responses map[abstract.Point]*libcosi.Response
	// Cosi is the cosi-structure that is used to sign
	cosi *libcosi.Cosi
	// Clients represents the total number of clients
	clients int
	// Signers represents the number of clients that signed
	signers int
}

// NewNextConfig prepares a new NextConfig
func NewNextConfig(sa *ServerKS) *NextConfig {
	return &NextConfig{
		cosi:      libcosi.NewCosi(network.Suite, sa.Private),
		responses: make(map[abstract.Point]*libcosi.Response),
		commits:   make(map[abstract.Point]*libcosi.Commitment),
	}
}

// NewConfig adds a new config and initialises all values to 0
func (nc *NextConfig) NewConfig(sa *ServerKS, conf *Config) (abstract.Secret, error) {
	dbg.Print("SA-config version is", sa.Config.Version)
	nc.config = conf
	nc.config.Version = sa.Config.Version + 1
	dbg.Print("Config-version is", nc.config.Version)
	nc.cosi = libcosi.NewCosi(network.Suite, sa.Private)
	nc.responses = make(map[abstract.Point]*libcosi.Response)
	nc.clients = len(sa.Config.Clients)
	nc.signers = 0

	// Calculating aggregate commit and add the message, which is the
	// hash of this configuration
	hashConfig := nc.config.Hash()
	ac := network.Suite.Point().Null()
	for _, c := range nc.commits {
		dbg.Print("Commitment", c.Commitment)
		ac.Add(ac, c.Commitment)
	}
	pb, err := ac.MarshalBinary()
	if err != nil {
		return nil, err
	}
	cipher := network.Suite.Cipher(pb)
	dbg.Print("Message is", hashConfig, pb)
	cipher.Message(nil, nil, hashConfig)
	challenge := network.Suite.Secret().Pick(cipher)
	dbg.Print("Challenge is", challenge)
	nc.config.Signature = &cosi.SignResponse{
		Sum:       hashConfig,
		Challenge: challenge,
	}
	nc.config.Signers = make([]*network.Entity, 0, nc.clients)

	// Empty the commitment-map
	nc.commits = make(map[abstract.Point]*libcosi.Commitment)

	return challenge, nil
}

// AddCommit stores that commit for the next challenge-creation
func (nc *NextConfig) AddCommit(e *network.Entity, c *libcosi.Commitment) {
	dbg.Print("Adding commit for", e.Public, c.Commitment)
	nc.commits[e.Public] = c
}

// Sign adds a response to the signature and checks if enough responses are
// present, which makes it create a signature.
// If enough responses are available, it returns true, else false
func (nc *NextConfig) AddResponse(e *network.Entity, r *libcosi.Response) bool {
	nc.responses[e.Public] = r
	nc.config.Signers = append(nc.config.Signers, e)
	dbg.Lvl3("Total responses / clients", len(nc.responses), nc.clients)
	if len(nc.responses) <= nc.clients/2 {
		dbg.Lvl2("Not enough signatures available - not yet signing")
		return false
	}

	// Create the aggregated Response
	aggregateResponse := network.Suite.Secret().Zero()
	for _, resp := range nc.responses {
		dbg.Print("Adding response", resp.Response)
		aggregateResponse = aggregateResponse.Add(aggregateResponse, resp.Response)
	}
	nc.config.Signature.Response = aggregateResponse
	return true
}

// GetConfig returns the config
func (nc *NextConfig) GetConfig() *Config {
	return nc.config
}
