package sda

import "github.com/dedis/cothority/network"

// Context represents the methods that are available to a service.
type Context struct {
	overlay *Overlay
	host    *Host
	servID  ServiceID
	manager *serviceManager
	network.Dispatcher
}

// newContext returns a Context pointing to all necessary elements.
func newContext(h *Host, o *Overlay, servID ServiceID, manager *serviceManager) *Context {
	return &Context{
		overlay:    o,
		host:       h,
		servID:     servID,
		manager:    manager,
		Dispatcher: network.NewBlockingDispatcher(),
	}
}

// NewTreeNodeInstance creates a TreeNodeInstance that is bound to a
// service instead of the Overlay.
func (c *Context) NewTreeNodeInstance(t *Tree, tn *TreeNode, protoName string) *TreeNodeInstance {
	return c.overlay.NewTreeNodeInstanceFromService(t, tn, ProtocolNameToID(protoName), c.servID)
}

// SendRaw sends a message to the ServerIdentity.
func (c *Context) SendRaw(si *network.ServerIdentity, msg interface{}) error {
	return c.host.Send(si, msg)
}

// ServerIdentity returns this Conode's identity.
func (c *Context) ServerIdentity() *network.ServerIdentity {
	return c.host.ServerIdentity
}

// ServiceID returns the service-id.
func (c *Context) ServiceID() ServiceID {
	return c.servID
}

// CreateProtocolService returns a ProtocolInstance bound to the service.
func (c *Context) CreateProtocolService(name string, t *Tree) (ProtocolInstance, error) {
	pi, err := c.overlay.CreateProtocolService(name, t, c.servID)
	return pi, err
}

// CreateProtocolSDA is like CreateProtocolService but doesn't bind it to a
// service, so it will be handled automatically by the SDA.
func (c *Context) CreateProtocolSDA(name string, t *Tree) (ProtocolInstance, error) {
	pi, err := c.overlay.CreateProtocolSDA(name, t)
	return pi, err
}

// RegisterProtocolInstance registers a new instance of a protocol using overlay.
func (c *Context) RegisterProtocolInstance(pi ProtocolInstance) error {
	return c.overlay.RegisterProtocolInstance(pi)
}

// ReportStatus returns all status of the services.
func (c *Context) ReportStatus() map[string]Status {
	return c.host.statusReporterStruct.ReportStatus()
}

// RegisterStatusReporter registers a new StatusReporter.
func (c *Context) RegisterStatusReporter(name string, s StatusReporter) {
	c.host.statusReporterStruct.RegisterStatusReporter(name, s)
}

// RegisterProcessor overrides the RegisterProcessor methods of the Dispatcher.
// It delegates the dispatching to the serviceManager.
func (c *Context) RegisterProcessor(p network.Processor, msgType network.PacketTypeID) {
	c.manager.RegisterProcessor(p, msgType)
}

// String returns the host it's running on.
func (c *Context) String() string {
	return c.host.ServerIdentity.String()
}
