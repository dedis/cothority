package sda

import (
	"testing"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

func TestConode_ProtocolRegisterName(t *testing.T) {
	c := NewLocalConode(0)
	defer c.Close()
	plen := len(c.protocols.Instantiators)
	require.True(t, plen > 0)
	id := c.ProtocolRegister("ConodeProtocol", NewConodeProtocol)
	require.NotNil(t, id)
	require.True(t, plen < len(c.protocols.Instantiators))
	_, err := c.ProtocolInstantiate(ProtocolID(uuid.Nil), nil)
	require.NotNil(t, err)
	// Test for not overwriting
	id2 := c.ProtocolRegister("ConodeProtocol", NewConodeProtocol2)
	require.Equal(t, id, id2)
}

func TestConode_GetService(t *testing.T) {
	c := NewLocalConode(0)
	defer c.Close()
	s := c.GetService("nil")
	require.Nil(t, s)
}

type ConodeProtocol struct {
	*TreeNodeInstance
}

// NewExampleHandlers initialises the structure for use in one round
func NewConodeProtocol(n *TreeNodeInstance) (ProtocolInstance, error) {
	return &ConodeProtocol{n}, nil
}

// NewExampleHandlers initialises the structure for use in one round
func NewConodeProtocol2(n *TreeNodeInstance) (ProtocolInstance, error) {
	return &ConodeProtocol{n}, nil
}

func (cp *ConodeProtocol) Start() error {
	return nil
}
