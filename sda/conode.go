package sda

import (
	"sync"

	"strings"

	"sort"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
)

// Conode is the structure responsible for holding information about the current
// state
type Conode struct {
	// Our entity (i.e. identity over the network)
	ServerIdentity *network.ServerIdentity
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
}

// NewConode returns a new Host that out of a private-key and its relating public
// key within the ServerIdentity. The host will create a default TcpRouter as Router.
func NewConode(e *network.ServerIdentity, pkey abstract.Scalar) *Conode {
	h := &Conode{
		ServerIdentity:       e,
		private:              pkey,
		statusReporterStruct: newStatusReporterStruct(),
	}

	var err error
	log.Lvl3("NewHost ", e.Address)
	h.Router, err = network.NewTCPRouter(e)
	if err != nil {
		panic(err)
	}
	h.overlay = NewOverlay(h)
	h.serviceManager = newServiceManager(h, h.overlay)
	h.statusReporterStruct.RegisterStatusReporter("Status", h)
	return h
}

// NewConodeWithRouter returns a fresh Host with a given Router.
func NewConodeWithRouter(e *network.ServerIdentity, pkey abstract.Scalar, r *network.Router) *Conode {
	h := &Conode{
		ServerIdentity:       e,
		private:              pkey,
		statusReporterStruct: newStatusReporterStruct(),
		Router:               r,
	}
	h.overlay = NewOverlay(h)
	h.serviceManager = newServiceManager(h, h.overlay)
	h.statusReporterStruct.RegisterStatusReporter("Status", h)
	return h
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
	//router := h.Router.GetStatus()
	//return router.Merge(m)

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
