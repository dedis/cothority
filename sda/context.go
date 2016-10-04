package sda

import "github.com/dedis/cothority/network"

// Context is the interface that is given to a Service
type Context struct {
	overlay *Overlay
	conode  *Conode
	servID  ServiceID
	manager *serviceManager
	network.Dispatcher
}

// defaultContext is the implementation of the Context interface. It is
// instantiated for each Service.

func newContext(h *Conode, o *Overlay, servID ServiceID, manager *serviceManager) *Context {
	return &Context{
		overlay:    o,
		conode:     h,
		servID:     servID,
		manager:    manager,
		Dispatcher: network.NewBlockingDispatcher(),
	}
}

// NewTreeNodeInstance is a Context method.
func (c *Context) NewTreeNodeInstance(t *Tree, tn *TreeNode, protoName string) *TreeNodeInstance {
	return c.overlay.NewTreeNodeInstanceFromService(t, tn, ProtocolNameToID(protoName), c.servID)
}

// SendRaw sends a message to the entity.
func (c *Context) SendRaw(si *network.ServerIdentity, msg interface{}) error {
	return c.conode.Send(si, msg)
}

// ServerIdentity returns the entity the service uses.
func (c *Context) ServerIdentity() *network.ServerIdentity {
	return c.conode.ServerIdentity
}

// ServiceID returns the service-id.
func (c *Context) ServiceID() ServiceID {
	return c.servID
}

// CreateProtocolService makes a TreeNodeInstance from the root-node of the tree and
// prepares for a 'name'-protocol. The ProtocolInstance has to be added later.
func (c *Context) CreateProtocolService(name string, t *Tree) (ProtocolInstance, error) {
	pi, err := c.overlay.CreateProtocolService(name, t, c.servID)
	return pi, err
}

// CreateProtocolSDA is like CreateProtocolService but doesn't bind a service to it,
// so it will be handled automatically by the SDA.
func (c *Context) CreateProtocolSDA(name string, t *Tree) (ProtocolInstance, error) {
	pi, err := c.overlay.CreateProtocolSDA(name, t)
	return pi, err
}

// RegisterProtocolInstance registers a new instance of a protocol using overlay.
func (c *Context) RegisterProtocolInstance(pi ProtocolInstance) error {
	return c.overlay.RegisterProtocolInstance(pi)
}

// ReportStatus is the status reporter but it works with context.
func (c *Context) ReportStatus() map[string]Status {
	return c.conode.statusReporterStruct.ReportStatus()
}

// RegisterStatusReporter registers the Status Reporter.
func (c *Context) RegisterStatusReporter(name string, s StatusReporter) {
	c.conode.statusReporterStruct.RegisterStatusReporter(name, s)
}

// RegisterProcessor overrides the RegisterProcessor methods of the dispatcher.
// It delegates the dispatching to the serviceManager.
func (c *Context) RegisterProcessor(p network.Processor, msgType network.PacketTypeID) {
	c.manager.RegisterProcessor(p, msgType)
}

// Service returns the corresponding service.
func (c *Context) Service(name string) Service {
	return c.manager.Service(name)
}

// String returns the host it's running on
func (c *Context) String() string {
	return c.conode.ServerIdentity.String()
}
