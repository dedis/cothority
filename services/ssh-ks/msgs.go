package ssh_ks

import (
	"errors"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/cosi"
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

// AddClient asks for addition of a client
type AddClient struct {
	Client *Client
}

// DelClient asks for the removal of a client
type DelClient struct {
	Client *Client
}

// Sign asks all servers to sign on the config, will return the new config
type Sign struct{}

// SignRet is the new signed config
type SignRet struct {
	Config *Config
}

// PropSig propagates the signature for a new config
type PropSig struct {
	Hash      []byte
	Version   int
	Signature *cosi.SignResponse
}

// StatusRet returns the success (empty string) or failure
type StatusRet struct {
	Error string
}

// FuncRegisterRet registers all messages to the network - not
// really necessary for the outgoing messages, but useful for
// external users
func FuncRegister() {
	var structs = []interface{}{
		ServerKS{},
		ClientKS{},
		GetServer{},
		GetServerRet{},
		GetConfig{},
		GetConfigRet{},
		AddServer{},
		DelServer{},
		AddClient{},
		DelClient{},
		Sign{},
		SignRet{},
		PropSig{},
		StatusRet{},
	}
	for _, s := range structs {
		network.RegisterMessageType(s)
	}
}

// NetworkGetServer asks for the Server at a given address
func NetworkGetServer(addr string) (*Server, error) {
	resp, err := NetworkSendAnonymous(addr, &GetServer{})
	if err != nil {
		return nil, err
	}
	conf, ok := resp.Msg.(GetServerRet)
	if !ok {
		return nil, errors.New("Didn't get Config back")
	}
	return conf.Server, nil
}
