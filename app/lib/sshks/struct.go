package sshks

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

type SSHksID skipchain.SkipBlockID

type SSHksConfig struct {
	ID SSHksID
	*skipchain.SkipBlock
	Clients  []*SSHClient
	Servers  []*SSHServer
	Majority int
}

// SSHClient represents one of the clients in the group
type SSHClient struct {
	abstract.Point
	SSHPub string
}

// NewSSHClient creates a new client from a public string representing
// its public SSH-key. It also returns the corresponding private key
func NewSSHClient(sshPub string) (*SSHClient, abstract.Secret) {
	pair := config.NewKeyPair(network.Suite)

	return &SSHClient{
		pair.Public,
		sshPub,
	}, pair.Secret
}

// SSHServer represents one of the servers in the group
type SSHServer struct {
	*network.Entity
	SSHPub string
}

// NewSSHServer creates a new server from a public string representing
// its public SSH-key and its Entity
func NewSSHServer(e *network.Entity, sshPub string) *SSHServer {
	return &SSHServer{
		e,
		sshPub,
	}
}
