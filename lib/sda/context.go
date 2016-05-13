package sda

import (
	"github.com/dedis/cothority/lib/network"
)

// Context is the interface that is given to an Service
type Context interface {
	NewTreeNodeInstance(*Tree, *TreeNode) *TreeNodeInstance
	RegisterProtocolInstance(ProtocolInstance) error
	SendRaw(*network.Entity, interface{}) error
	CreateProtocol(t *Tree, name string) (ProtocolInstance, error)
	Address() string
	Entity() *network.Entity
}

// defaultContext is the implementation of the Context interface. It is
// instantiated for each Service.
type defaultContext struct {
	*Overlay
	*Host
	servID ServiceID
}

func newDefaultContext(h *Host, o *Overlay, servID ServiceID) *defaultContext {
	return &defaultContext{
		Overlay: o,
		Host:    h,
		servID:  servID,
	}
}

// NewTreeNodeInstance implements the Context interface method
func (dc *defaultContext) NewTreeNodeInstance(t *Tree, tn *TreeNode) *TreeNodeInstance {
	return dc.Overlay.NewTreeNodeInstanceFromService(t, tn, dc.servID)
}

// SendRaw sends a message to the entity
func (dc *defaultContext) SendRaw(e *network.Entity, msg interface{}) error {
	return dc.Host.SendRaw(e, msg)
}

// Entity returns the entity the service uses
func (dc *defaultContext) Entity() *network.Entity {
	return dc.Host.Entity
}
