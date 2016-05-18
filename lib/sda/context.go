package sda

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/dbg"
)

// Context is the interface that is given to a Service
type Context interface {
	NewTreeNodeInstance(*Tree, *TreeNode) *TreeNodeInstance
	RegisterProtocolInstance(ProtocolInstance) error
	SendRaw(*network.Entity, interface{}) error
	CreateProtocol(*Tree, string) (ProtocolInstance, error)
	CreateProtocolAuto(*Tree, string) (ProtocolInstance, error)
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

func (dc *defaultContext) CreateProtocol(t *Tree, name string) (ProtocolInstance, error) {
	pi, err := dc.Overlay.CreateProtocolService(dc.servID, t, name)
	dbg.Printf("Storing service id +%v", pi.Token().ServiceID)
	return pi, err
}

func (dc *defaultContext) CreateProtocolAuto(t *Tree, name string) (ProtocolInstance, error) {
	pi, err := dc.Overlay.CreateProtocol(t, name)
	return pi, err
}
