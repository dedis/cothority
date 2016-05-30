package sda

import "github.com/dedis/cothority/lib/network"

// Context is the interface that is given to a Service
type Context interface {
	NewTreeNodeInstance(*Tree, *TreeNode, string) *TreeNodeInstance
	RegisterProtocolInstance(ProtocolInstance) error
	SendRaw(*network.Entity, interface{}) error
	CreateProtocolService(*Tree, string) (ProtocolInstance, error)
	CreateProtocolSDA(*Tree, string) (ProtocolInstance, error)
	Address() string
	Entity() *network.Entity
	GetID() ServiceID
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
func (dc *defaultContext) NewTreeNodeInstance(t *Tree, tn *TreeNode, protoName string) *TreeNodeInstance {
	return dc.Overlay.NewTreeNodeInstanceFromService(t, tn, ProtocolNameToID(protoName), dc.servID)
}

// SendRaw sends a message to the entity
func (dc *defaultContext) SendRaw(e *network.Entity, msg interface{}) error {
	return dc.Host.SendRaw(e, msg)
}

// Entity returns the entity the service uses
func (dc *defaultContext) Entity() *network.Entity {
	return dc.Host.Entity
}

// GetID returns the service-id
func (dc *defaultContext) GetID() ServiceID {
	return dc.servID
}

// CreateProtocolService makes a TreeNodeInstance from the root-node of the tree and
// prepares for a 'name'-protocol. The ProtocolInstance has to be added later.
func (dc *defaultContext) CreateProtocolService(t *Tree, name string) (ProtocolInstance, error) {
	pi, err := dc.Overlay.CreateProtocolService(dc.servID, t, name)
	return pi, err
}

// CreateProtocolSDA is like CreateProtocolService but doesn't bind a service to it,
// so it will be handled automatically by the SDA.
func (dc *defaultContext) CreateProtocolSDA(t *Tree, name string) (ProtocolInstance, error) {
	pi, err := dc.Overlay.CreateProtocolSDA(t, name)
	return pi, err
}
