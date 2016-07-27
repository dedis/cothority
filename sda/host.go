package sda

import (
	"sync"

	"strings"

	"sort"

	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// Host is the structure responsible for holding information about the current
// state
type Host struct {
	// Our entity (i.e. identity over the network)
	ServerIdentity *network.ServerIdentity
	// Our private-key
	private abstract.Scalar
	suite   abstract.Suite
	Router
	// Overlay handles the mapping from tree and entityList to ServerIdentity.
	// It uses tokens to represent an unique ProtocolInstance in the system
	overlay *Overlay
	// The open connections
	connections map[network.ServerIdentityID]network.SecureConn
	// lock associated to access trees
	treesLock            sync.Mutex
	serviceManager       *serviceManager
	statusReporterStruct *statusReporterStruct
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(e *network.ServerIdentity, pkey abstract.Scalar) *Host {
	h := &Host{
		ServerIdentity:       e,
		connections:          make(map[network.ServerIdentityID]network.SecureConn),
		suite:                network.Suite,
		statusReporterStruct: newStatusReporterStruct(),
		Router:               NewTcpRouter(e, pkey),
	}

	h.overlay = NewOverlay(h)
	h.serviceManager = newServiceManager(h, h.overlay)
	h.statusReporterStruct.RegisterStatusReporter("Status", h)
	return h
}

func NewHostWithRouter(e *network.ServerIdentity, pkey abstract.Scalar, r Router) *Host {
	h := &Host{
		ServerIdentity:       e,
		connections:          make(map[network.ServerIdentityID]network.SecureConn),
		suite:                network.Suite,
		statusReporterStruct: newStatusReporterStruct(),
		Router:               r,
	}
	h.overlay = NewOverlay(h)
	h.serviceManager = newServiceManager(h, h.overlay)
	h.statusReporterStruct.RegisterStatusReporter("Status", h)
	return h
}

// AddTree registers the given Tree struct in the underlying overlay.
// Useful for unit-testing only.
// XXX probably move into the tests.
func (h *Host) AddTree(t *Tree) {
	h.overlay.RegisterTree(t)
}

// AddRoster registers the given Roster in the underlying overlay.
// Useful for unit-testing only.
// XXX probably move into the tests.
func (h *Host) AddRoster(el *Roster) {
	h.overlay.RegisterRoster(el)
}

// Suite can (and should) be used to get the underlying abstract.Suite.
// Currently the suite is hardcoded into the network library.
// Don't use network.Suite but Host's Suite function instead if possible.
func (h *Host) Suite() abstract.Suite {
	return h.suite
}

// SetupHostsMock can be used to create a Host mock for testing.
func SetupHostsMock(s abstract.Suite, addresses ...string) []*Host {
	var hosts []*Host
	for _, add := range addresses {
		h := newHostMock(s, add)
		h.ListenAndBind()
		h.StartProcessMessages()
		hosts = append(hosts, h)
	}
	return hosts
}

func newHostMock(s abstract.Suite, address string) *Host {
	kp := config.NewKeyPair(s)
	en := network.NewServerIdentity(kp.Public, address)
	return NewHost(en, kp.Secret)
}

// GetStatus is a function that returns the status report of the server.
func (h *Host) GetStatus() Status {
	m := make(map[string]string)
	a := ServiceFactory.RegisteredServicesName()
	sort.Strings(a)
	m["Available_Services"] = strings.Join(a, ",")
	router := h.Router.GetStatus()
	return router.Merge(m)

}

func (h *Host) Close() error {
	h.Router.Close()
	h.overlay.Close()
	return nil
}
