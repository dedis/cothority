package sda

import (
	"sync"

	"strings"

	"sort"

	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
)

// Conode is the structure responsible for holding information about the current
// state
type Conode struct {
	// Our private-key
	private abstract.Scalar
	*network.Router
	// Overlay handles the mapping from tree and entityList to ServerIdentity.
	// It uses tokens to represent an unique ProtocolInstance in the system
	overlay *Overlay
	// lock associated to access trees
	treesLock            sync.Mutex
	serviceManager       *serviceManager
	statusReporterStruct *statusReporterStruct
	// protocols holds a map of all available protocols and how to create an
	// instance of it
	protocols *protocolStorage
}

// NewConode returns a fresh Host with a given Router.
func NewConode(r *network.Router, pkey abstract.Scalar) *Conode {
	c := &Conode{
		private:              pkey,
		statusReporterStruct: newStatusReporterStruct(),
		Router:               r,
		protocols:            newProtocolStorage(),
	}
	c.overlay = NewOverlay(c)
	c.serviceManager = newServiceManager(c, c.overlay)
	c.statusReporterStruct.RegisterStatusReporter("Status", c)
	for name, inst := range protocols.instantiators {
		log.Lvl4("Registering global protocol", name)
		c.ProtocolRegister(name, inst)
	}
	return c
}

// NewConodeTCP returns a new Host that out of a private-key and its relating public
// key within the ServerIdentity. The host will create a default TcpRouter as Router.
func NewConodeTCP(e *network.ServerIdentity, pkey abstract.Scalar) *Conode {
	r, err := network.NewTCPRouter(e)
	log.ErrFatal(err)
	return NewConode(r, pkey)
}

// Suite can (and should) be used to get the underlying abstract.Suite.
// Currently the suite is hardcoded into the network library.
// Don't use network.Suite but Host's Suite function instead if possible.
func (c *Conode) Suite() abstract.Suite {
	return network.Suite
}

// GetStatus is a function that returns the status report of the server.
func (c *Conode) GetStatus() Status {
	m := make(map[string]string)
	a := ServiceFactory.RegisteredServiceNames()
	sort.Strings(a)
	m["Available_Services"] = strings.Join(a, ",")
	return m
}

// Close closes the overlay and the Router
func (c *Conode) Close() error {
	c.overlay.Close()
	err := c.Router.Stop()
	log.Lvl3("Host Close ", c.ServerIdentity.Address, "listening?", c.Router.Listening())
	return err

}

// Address returns the address used by the Router.
func (c *Conode) Address() network.Address {
	return c.ServerIdentity.Address
}

// GetService returns the service with the given name.
func (c *Conode) GetService(name string) Service {
	return c.serviceManager.Service(name)
}

// ProtocolRegister will sign up a new protocol to this Conode.
// It returns the ID of the protocol.
func (c *Conode) ProtocolRegister(name string, protocol NewProtocol) (ProtocolID, error) {
	return c.protocols.Register(name, protocol)
}

// ProtocolInstantiate instantiate a protocol from its ID
func (c *Conode) ProtocolInstantiate(protoID ProtocolID, tni *TreeNodeInstance) (ProtocolInstance, error) {
	fn, ok := c.protocols.instantiators[c.protocols.ProtocolIDToName(protoID)]
	if !ok {
		return nil, errors.New("No protocol constructor with this ID")
	}
	return fn(tni)
}
