package ssh_ks

import (
	"bytes"
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/cosi"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"io/ioutil"
	"os"
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
	// DirSSHD is the directory where the server's public key is stored
	DirSSHD string
	// DirSSH is the directory where the authorized_keys will be written to
	DirSSH string
	// host represents the actual running host
	host *sda.Host
}

// NewCoNode creates a new node of the cothority and initializes the
// Config-structures. It doesn't start the node
func NewServerApp(key *config.KeyPair, addr, dirSSHD, dirSSH string) (*ServerApp, error) {
	sshdPub, err := ioutil.ReadFile(dirSSHD + "/ssh_host_rsa_key.pub")
	if err != nil {
		return nil, err
	}
	srv := NewServer(key.Public, addr, string(sshdPub))
	c := &ServerApp{
		This:    srv,
		Private: key.Secret,
		Config:  NewConfig(0),
		DirSSHD: dirSSHD,
		DirSSH:  dirSSH,
	}
	c.AddServer(srv)
	return c, nil
}

func ReadServerApp(f string) (*ServerApp, error) {
	file := expandHDir(f)
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
	sa := msg.(ServerApp)
	return &sa, err
}

func (sa *ServerApp) WriteConfig(file string) error {
	b, err := network.MarshalRegisteredType(sa)
	if err != nil {
		return err
	}
	ioutil.WriteFile(expandHDir(file), b, 0660)
	return nil
}

// AddServer inserts a server in the configuration-list
func (sa *ServerApp) AddServer(s *Server) error {
	sa.Config.AddServer(s)
	return nil
}

// DelServer removes a server from the configuration-list
func (sa *ServerApp) DelServer(s *Server) error {
	sa.Config.DelServer(s)
	return nil
}

// Start opens the port indicated for listening
func (sa *ServerApp) Start() error {
	sa.host = sda.NewHost(sa.This.Entity, sa.Private)
	sa.host.RegisterMessage(GetServer{}, sa.FuncGetServer)
	sa.host.RegisterMessage(GetConfig{}, sa.FuncGetConfig)
	sa.host.RegisterMessage(AddServer{}, sa.FuncAddServer)
	sa.host.RegisterMessage(DelServer{}, sa.FuncDelServer)
	sa.host.RegisterMessage(AddClient{}, sa.FuncAddClient)
	sa.host.RegisterMessage(DelClient{}, sa.FuncDelClient)
	sa.host.RegisterMessage(Sign{}, sa.FuncSign)
	sa.host.RegisterMessage(PropSig{}, sa.FuncPropSig)
	cosi.AddCosiApp(sa.host)
	sa.host.Listen()
	sa.host.StartProcessMessages()
	return nil
}

// Stop closes the connection
func (sa *ServerApp) Stop() error {
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
func (sa *ServerApp) Check() error {
	for _, s := range sa.Config.Servers {
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
func (sa *ServerApp) Sign() error {
	sa.Config.Version += 1
	sa.Config.Signature = nil
	msg := sa.Config.Hash()
	msg2 := sa.Config.Hash()
	if bytes.Compare(msg, msg2) != 0 {
		dbg.Fatal("Hashing is different")
	}
	var err error
	sa.Config.Signature, err = cosi.SignStatement(bytes.NewReader(msg),
		sa.Config.EntityList(sa.This.Entity))
	if err != nil {
		return err
	}
	// And send all updated signatures to the other servers
	for _, srv := range sa.Config.Servers {
		if srv != sa.This {
			ps := &PropSig{
				Version:   sa.Config.Version,
				Hash:      sa.Config.Hash(),
				Signature: sa.Config.Signature,
			}
			dbg.Lvl3("Seding propagation to", srv.Entity.Addresses[0])
			resp, err := networkSend(sa.Private, srv.Entity, ps)
			err = errMsg(resp, err)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// FuncGetServer returns our Server
func (sa *ServerApp) FuncGetServer(*network.Message) network.ProtocolMessage {
	return &GetServerRet{sa.This}
}

// FuncGetConfig returns our Config
func (sa *ServerApp) FuncGetConfig(*network.Message) network.ProtocolMessage {
	return &GetConfigRet{sa.Config}
}

// FuncAddServer adds a given server to the configuration
func (sa *ServerApp) FuncAddServer(data *network.Message) network.ProtocolMessage {
	req, ok := data.Msg.(AddServer)
	if !ok {
		return &StatusRet{"Didn't get a server"}
	}
	dbg.Lvl3("Adding server", req.Server, "to", sa.This)
	sa.AddServer(req.Server)
	return &StatusRet{""}
}

// FuncDelServer removes a given server from the configuration
func (sa *ServerApp) FuncDelServer(data *network.Message) network.ProtocolMessage {
	req, ok := data.Msg.(DelServer)
	if !ok {
		return &StatusRet{"Didn't get a server"}
	}
	if sa.This.Entity.Addresses[0] == req.Server.Entity.Addresses[0] {
		return &StatusRet{"Cannot delete own address"}
	}
	sa.DelServer(req.Server)
	return &StatusRet{""}
}

// FuncAddClient adds a given client to the configuration
func (sa *ServerApp) FuncAddClient(data *network.Message) network.ProtocolMessage {
	req, ok := data.Msg.(AddClient)
	if !ok {
		return &StatusRet{"Didn't get a client"}
	}
	dbg.Lvl3("Adding client", req.Client, "to", sa.This)
	if req.Client.SSHpub == "" {
		return &StatusRet{"Client with empty SSHpub is not allowed"}
	}
	sa.Config.AddClient(req.Client)
	return &StatusRet{""}
}

// FuncDelServer removes a given server from the configuration
func (sa *ServerApp) FuncDelClient(data *network.Message) network.ProtocolMessage {
	req, ok := data.Msg.(DelClient)
	if !ok {
		return &StatusRet{"Didn't get a client"}
	}
	if sa.This.Entity.Addresses[0] == req.Client.SSHpub {
		return &StatusRet{"Cannot delete own address"}
	}
	sa.Config.DelClient(req.Client)
	return &StatusRet{""}
}

// FuncSign asks all servers to sign the new configuration
func (sa *ServerApp) FuncSign(data *network.Message) network.ProtocolMessage {
	dbg.Lvl3("Starting to sign")
	status := &StatusRet{}
	err := sa.Sign()
	if err != nil {
		dbg.Error(err)
		status = &StatusRet{err.Error()}
	}
	return status
}

// FuncPropSig checks the hash and if it matches updates the signature
func (sa *ServerApp) FuncPropSig(data *network.Message) network.ProtocolMessage {
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
func (sa *ServerApp) CreateAuth() error {
	lines := make([]string, 0, len(sa.Config.Clients))
	for _, c := range sa.Config.Clients {
		lines = append(lines, c.SSHpub)
	}
	return ioutil.WriteFile(sa.DirSSH+"/authorized_keys",
		[]byte(strings.Join(lines, "\n")), 0600)
}
