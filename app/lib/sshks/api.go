package sshks

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

/*
app/lib/sshks

client
has private-key
has shared config
proposes, checks and votes for new configs
updates config

server
has private-key
has shared config
updates configs
 */

// App is the communication with the app-skipchain of the Cothority
type App struct {
	*sda.Client
	Config  *SSHksConfig
	Private abstract.Secret
}

// Server is for the communication of the SSHksServer to the Cothority
type Server struct{
	*sda.Client
}

// NewClient instantiates a new SSHksConfig client
func NewClient(majority int, cothority *network.Entity) *App {
	return &App{Client: sda.NewClient(ServiceName)}
}

// NewClientFromStream reads the configuration of that client from
// any stream
func NewClientFromStream() (*App, error)

// SaveClientToStream stores the configuration of the client to a stream
func (c *App) SaveClientToStream() error {
	return nil
}

// ConfigUpdate asks if there is any new config available that has already
// been approved by others
func (c *App) ConfigUpdate() error {
	return nil
}

// ConfigNewPropose is sent from a client to the Cothority, signed with its
// private key
func (c *App) ConfigNewPropose(*SSHksConfig) error {
	return nil
}

// ConfigNewCheck verifies if there is a new configuration awaiting that
// needs approval from clients
func (c *App) ConfigNewCheck() (*SSHksConfig, error) {
	return nil
}

// ConfigNewVote sends a vote (accept or reject) with regard to a new configuration
func (c *App) ConfigNewVote(ConfigID, accept bool) error {
	return nil
}
