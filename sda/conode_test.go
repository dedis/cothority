package sda

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConode_ProtocolRegisterName(t *testing.T) {
	c := NewLocalConode(0)
	defer c.Close()
	plen := len(c.protocolNames)
	require.True(t, plen > 0)
	id := c.ProtocolRegisterName("ConodeProtocol", NewConodeProtocol)
	require.NotNil(t, id)
	require.True(t, plen < len(c.protocolNames))
}

type ConodeProtocol struct {
	*TreeNodeInstance
}

// NewExampleHandlers initialises the structure for use in one round
func NewConodeProtocol(n *TreeNodeInstance) (ProtocolInstance, error) {
	return &ConodeProtocol{n}, nil
}

func (cp *ConodeProtocol) Start() error {
	return nil
}
