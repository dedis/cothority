package sshks

import (
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
	sa.host.RegisterExternalMessage(PropConfig{}, sa.FuncPropConfig)
	sa.host.RegisterExternalMessage(ProposeClient{}, sa.FuncProposeClient)
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
	dbg.Lvl3(data.Entity, *sa.Config)
	if sa.unknownClient(data.Entity) {
		return &StatusRet{"Refusing unknown client"}
	}
	comm, ok := data.Msg.(SendFirstCommit)
	if !ok {
		return &StatusRet{"Didn't get a commit"}
	}
	sa.NextConfig.AddCommit(data.Entity, comm.Commitment)
	err := sa.PropagateConfig()
	if err != nil {
		return &StatusRet{"Couldn't propagate commits: " + err.Error()}
	}
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
	dbg.Lvl3("Got new config", *conf.Config)
	chal, err := sa.NextConfig.NewConfig(sa, conf.Config)
	if err != nil {
		return &SendNewConfigRet{}
	}
	err = sa.PropagateConfig()
	if err != nil {
		return &SendNewConfigRet{}
	}
	return &SendNewConfigRet{chal}
}

// FuncGetConfig returns our Config
func (sa *ServerKS) FuncGetConfig(*network.Message) network.ProtocolMessage {
	var newconf *Config
	if sa.NextConfig.Config != nil {
		if sa.NextConfig.Config.Version > sa.Config.Version {
			newconf = sa.NextConfig.Config
			dbg.Lvl3("Adding new config", *newconf)
		}
	}
	return &GetConfigRet{
		Config:    sa.Config,
		NewConfig: newconf,
	}
}

// FuncResponse sends a response to an accepted config. If the server receives
// enough responses, it will sign the message
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
		sa.Config = sa.NextConfig.Config
		dbg.Lvl3("Storing new config version", sa.Config.Version, sa.Config.Clients)
	}
	sa.NextConfig.AddCommit(data.Entity, response.NextCommitment)
	err := sa.PropagateConfig()
	if err != nil {
		dbg.Error(err)
	}
	dbg.Lvl3("Ok is", ok)
	if ok {
		err := sa.Config.VerifySignature()
		if err != nil {
			dbg.Error("Signature is wrong - sending anyway",
				err)
		}
		return &ResponseRet{
			ClientsTot:    sa.NextConfig.Clients,
			ClientsSigned: sa.NextConfig.Signers,
			Config:        sa.Config,
		}
	}

	return &ResponseRet{sa.NextConfig.Clients, sa.NextConfig.Signers, nil}
}

// FuncPropConfig stores the new config and also all new commits
func (sa *ServerKS) FuncPropConfig(data *network.Message) network.ProtocolMessage {
	if sa.unknownServer(data.Entity) {
		return &StatusRet{"Refusing unknown server"}
	}
	pc, ok := data.Msg.(PropConfig)
	if !ok {
		return &StatusRet{"Didn't get a config"}
	}
	err := pc.Config.VerifySignature()
	if err != nil {
		return &StatusRet{"Wrong signature"}
	}
	sa.Config = pc.Config
	if pc.NextConfig != nil {
		sa.NextConfig = pc.NextConfig.InitMaps()
	}
	return &StatusRet{""}
}

// FuncProposeClient makes a new config and sends it to all other servers
func (sa *ServerKS) FuncProposeClient(data *network.Message) network.ProtocolMessage {
	pc, ok := data.Msg.(ProposeClient)
	if !ok {
		return &StatusRet{"Didn't get a client"}
	}
	conf, err := sa.Config.Copy()
	if err != nil {
		return &StatusRet{"Couldn't copy config"}
	}
	conf.AddClient(pc.Client)
	_, err = sa.NextConfig.NewConfig(sa, conf)
	if err != nil {
		return &StatusRet{"Couldn't create new config"}
	}
	err = sa.PropagateConfig()
	if err != nil {
		return &StatusRet{err.Error()}
	}
	return &StatusRet{""}
}

// PropagateConfig sends the new config to all other servers
func (sa *ServerKS) PropagateConfig() error {
	pc := &PropConfig{
		Config:     sa.Config,
		NextConfig: sa.NextConfig,
	}
	for _, s := range sa.Config.Servers {
		if !s.Entity.Public.Equal(sa.This.Entity.Public) {
			dbg.Lvl3("Sending new config to", s.Entity.String())
			resp, err := NetworkSend(sa.Private, s.Entity, pc)
			errm := ErrMsg(resp, err)
			dbg.Lvlf3("Response is %+v %+v - %+v", resp, err, errm)
			if errm != nil {
				return errm
			}
		}
	}
	return nil
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

// unknownServer returns true if the message comes from an
// unknown server
func (sa *ServerKS) unknownServer(e *network.Entity) bool {
	dbg.Lvl3("My servers:", sa.Config.Servers, "server asking:", *e)
	if len(sa.Config.Servers) == 1 {
		// If we're a new server, accept everything
		return false
	}
	_, known := sa.Config.Servers[e.Public.String()]
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
	Config *Config
	// Commits is a map of public-keys to pre-computed commits from the clients
	Commits map[abstract.Point]*libcosi.Commitment
	// Responses holds all responses received so far
	Responses map[abstract.Point]*libcosi.Response
	// Cosi is the cosi-structure that is used to sign
	Cosi *libcosi.Cosi
	// Clients represents the total number of clients
	Clients int
	// Signers represents the number of clients that signed
	Signers int
}

// NewNextConfig prepares a new NextConfig
func NewNextConfig(sa *ServerKS) *NextConfig {
	nc := &NextConfig{
		Cosi:   libcosi.NewCosi(network.Suite, sa.Private),
		Config: NewConfig(0),
	}
	return nc.InitMaps()
}

// InitMaps is used when the NextConfig is Unmarshaled, because protobuf
// doesn't initialize 0-length maps.
func (nc *NextConfig) InitMaps() *NextConfig {
	if len(nc.Responses) == 0 {
		nc.Responses = make(map[abstract.Point]*libcosi.Response)
	}
	if len(nc.Commits) == 0 {
		nc.Commits = make(map[abstract.Point]*libcosi.Commitment)
	}
	return nc
}

// NewConfig adds a new config and initialises all values to 0
func (nc *NextConfig) NewConfig(sa *ServerKS, conf *Config) (abstract.Secret, error) {
	dbg.Lvl3("SA-config version is", sa.Config.Version)
	nc.Config = conf
	nc.Config.Version = sa.Config.Version + 1
	dbg.Lvl3("Config-version is", nc.Config.Version)
	nc.Cosi = libcosi.NewCosi(network.Suite, sa.Private)
	nc.Responses = make(map[abstract.Point]*libcosi.Response)
	nc.Clients = len(sa.Config.Clients)
	nc.Signers = 0

	// Calculating aggregate commit and add the message, which is the
	// hash of this configuration
	hashConfig := nc.Config.Hash()
	ac := network.Suite.Point().Null()
	for _, c := range nc.Commits {
		dbg.Lvl3("Commitment", c.Commitment)
		ac.Add(ac, c.Commitment)
	}
	pb, err := ac.MarshalBinary()
	if err != nil {
		return nil, err
	}
	cipher := network.Suite.Cipher(pb)
	dbg.Lvl3("Message is", hashConfig, pb)
	cipher.Message(nil, nil, hashConfig)
	challenge := network.Suite.Secret().Pick(cipher)
	dbg.Lvl3("Challenge is", challenge)
	nc.Config.Signature = &cosi.SignResponse{
		Sum:       hashConfig,
		Challenge: challenge,
	}
	nc.Config.Signers = make([]*network.Entity, 0, nc.Clients)

	// Empty the commitment-map
	nc.Commits = make(map[abstract.Point]*libcosi.Commitment)

	return challenge, nil
}

// AddCommit stores that commit for the next challenge-creation
func (nc *NextConfig) AddCommit(e *network.Entity, c *libcosi.Commitment) {
	dbg.Lvl3("Adding commit for", e.Public, c.Commitment)
	nc.Commits[e.Public] = c
}

// Sign adds a response to the signature and checks if enough responses are
// present, which makes it create a signature.
// If enough responses are available, it returns true, else false
func (nc *NextConfig) AddResponse(e *network.Entity, r *libcosi.Response) bool {
	nc.Responses[e.Public] = r
	nc.Config.Signers = append(nc.Config.Signers, e)
	dbg.Lvl3("Total responses / clients", len(nc.Responses), nc.Clients)
	if len(nc.Responses) <= nc.Clients/2 {
		dbg.Lvl2("Not enough signatures available - not yet signing")
		return false
	}

	// Create the aggregated Response
	aggregateResponse := network.Suite.Secret().Zero()
	for _, resp := range nc.Responses {
		dbg.Lvl3("Adding response", resp.Response)
		aggregateResponse = aggregateResponse.Add(aggregateResponse, resp.Response)
	}
	if nc.Config.Signature == nil {
		nc.Config.Signature = &cosi.SignResponse{}
	}
	nc.Config.Signature.Response = aggregateResponse
	return true
}

// GetConfig returns the config
func (nc *NextConfig) GetConfig() *Config {
	return nc.Config
}
