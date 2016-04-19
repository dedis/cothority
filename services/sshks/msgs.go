package sshks

import (
	"errors"

	libcosi "github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
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
	// NewConfig is nil if there is no config to be confirmed
	NewConfig *Config
}

// Signing-related messages:
// 1 - SendNewConfig the new message to be signed
// 2 - GetNewConfig called by the other clients
// 3 - Sign sent by more than 50% of the clients
// Once the server receives more than 50% of responses, it will store
// that config as the current config, then the clients can do
// 4 - GetConfig to receive the current config signed by more than 50% of the clients

// The very first commitment needs to be sent here - it will return
// a ResponseRet
type SendFirstCommit struct {
	Commitment *libcosi.Commitment
}

// SendNewConfig sends the config to be proposed to other clients. It will reply
// with the challenge that all clients have to respond to
type SendNewConfig struct {
	Config *Config
}

// SendNewConfigRet contains the challenge the clients have to reply to
type SendNewConfigRet struct {
	Challenge abstract.Secret
}

// GetNewConfig asks for the new configuration
type GetNewConfig struct{}

// GetNewConfigRet replies with the new config and a challenge. This is phase 3
// of the CoSi-protocol and done by combining the pre-computed commits that
// are stored in the server. Now this client can 'respond' to
// (phase 4 of the CoSi-protocol)
// If there is no new configuration, both are 'nil'
type GetNewConfigRet struct {
	NewConfig *Config
	Challenge *libcosi.Challenge
}

// Response sends one response (4th phase of the CoSi-protocol) to the server
// plus a commitment (2nd phase of the CoSi-protocol) for the NEXT round. New
// clients can send this with a 'Response' = nil to store their commitment.
type Response struct {
	// Response for the new config
	Response *libcosi.Response
	// NextCommitment is the new commitment for the NEXT round, the commitment
	// for the actual round should already be on the server
	NextCommitment *libcosi.Commitment
}

// ResponseRet returns the status of the signature
type ResponseRet struct {
	// ClientsTot how many clients in total are defined
	ClientsTot int
	// ClientsSigned how many clients already signed (including this)
	ClientsSigned int
	// Config is nil if not enough clients signed off yet
	Config *Config
}

// Server-internal messages to be sent between servers

// PropConfig propagates the new config - it also needs to send the latest
// commit-map of pre-computed commits, so clients can sign anywhere
type PropConfig struct {
	Config *Config
	// Commits is a map of public-keys to pre-computed commits from the clients
	Commits map[abstract.Point]*libcosi.Commitment
}

// StatusRet returns the success (empty string) or failure
type StatusRet struct {
	Error string
}

// FuncRegister registers all messages to the network - not
// really necessary for the outgoing messages, but useful for
// external users
func FuncRegister() {
	var structs = []interface{}{
		SendFirstCommit{},
		SendNewConfig{},
		SendNewConfigRet{},
		ServerKS{},
		ClientKS{},
		GetServer{},
		GetServerRet{},
		GetConfig{},
		GetConfigRet{},
		Response{},
		ResponseRet{},
		PropConfig{},
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
