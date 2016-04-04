// Package sda has the service.go file which contains the code that glues the innerpart of SDA with the external
// part. Basically, you have the definition of a Protocol which anybody can
// implement, this is what sda is for: to run protocols.
//
// A Protocol in sda is supposed to be a short term struct that runs its
// algorithm and then finishes. SDA handles the rounds and the dispatching.
// Multiple protocols can run in parallel with the same Tree / EntityList etc.
// You  can registers Protocol at init time using `sda.ProtocolRegister`.
// If you need a long term object to store longterm information that Protocol
// can use (for example, the hash of the last signature a given protocol has
// issued to create a blockchain), you should implement the Service interface.
//
// A service is a longterm running platform that performs two main functions:
// * First, it creates the ProtocolInstances out of a Node. SDA will request the
// right service each time it needs to create a new one. The service has to
// provide any external additional information to the protocol so it can work
// knowing everything it needs to know.
// * Secondly, a Service responds to requests made by clients through a CLI
// using the API. The service will be responsible to create a ProtocolInstance
// out of the client's request with the right Tree + EntityList and starts the
// protocol.
// The service can reply to requests coming also from others services or others
// nodes; the Request is only a wrapper around JSON object.
// All services are registered at init time and directly startup in the main
// function.
package sda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
	"os"
	"path"
)

// ProtocolInstance is the interface that instances have to use in order to be
// recognized as protocols
type ProtocolInstance interface {
	// Start is called when a leader has created its tree configuration and
	// wants to start a protocol, it calls host.StartProtocol(protocolID), that
	// in turns instantiate a new protocol (with a fresh token), and then call
	// Start on it.
	Start() error
	// Dispatch is called as a go-routine and can be used to handle channels
	Dispatch() error
	// Shutdown cleans up the resources used by this protocol instance
	Shutdown() error
}

// NewProtocol is a convenience to represent the cosntructor function of a
// ProtocolInstance
type NewProtocol func(*Node) (ProtocolInstance, error)

// ProtocolConstructor is an interface that can instantiate a given protocol
type ProtocolConstructor interface {
	NewProtocol(*Node) (ProtocolInstance, error)
}

// Service is a generic interface to define any type of services.
type Service interface {
	// ProtocolConstructor.NewProtocol is called by sda when it receives a packet for an
	// non-existing Node (first contact). The service should provide the
	// ProtocolInstance with every (external) information it might need.
	ProtocolConstructor

	// ProcessRequest is the function that will be called when a external client
	// using the CLI will contact this service with a request packet.
	// Each request has a field ServiceID, so each time the Host (dispatcher)
	// receives a request, it looks whether it knows the Service it is for and
	// then dispatch it through ProcessRequest.
	ProcessRequest(*network.Entity, *Request)
}

// ProtocolID is an identifier for the different protocols.
type ProtocolID uuid.UUID

// String returns a string representation of this ProtocolID
func (p *ProtocolID) String() string {
	return uuid.UUID(*p).String()
}

// EmptyProtocolID is a nil - empty ProtocolID which should not correspond to
// any particular protocol
var EmptyProtocolID = ProtocolID(uuid.Nil)

// protocolFactory stores all the ProtocolConstructor together. It can
// instantiate any registered protocol by name.
type protocolFactory struct {
	constructors map[ProtocolID]ProtocolConstructor
	// translations between ProtocolID and the name
	translations map[string]ProtocolID
	// the reverse translation for easy debugging
	reverse map[ProtocolID]string
}

// The global factory that can be used to instantiate any protocol
var ProtocolFactory = &protocolFactory{
	constructors: make(map[ProtocolID]ProtocolConstructor),
	translations: make(map[string]ProtocolID),
	reverse:      make(map[ProtocolID]string)}

// RegisterNewProtocol takes the name of the protocol and a NewProtocol function
// that will be stored.
func (pf *protocolFactory) RegisterNewProtocol(name string, new NewProtocol) {
	// creates a default constructor out of the NewProtocol func
	dc := &defaultConstructor{new}
	pf.register(name, dc)
	dbg.Lvl2("RegisterNewProtocol:", name)
}

// RegisterProtocolConstructor take the name of the protocol and the
// ProtocolConstructor used to instantiate it.
func (pf *protocolFactory) RegisterProtocolConstructor(name string, cons ProtocolConstructor) {
	dbg.Lvl2("RegisterNewProtocolConstructor:", name)
	pf.register(name, cons)
}

// RegisterNewProtocol is a wrapper around protocolFactory.RegisterNewProtocol
func RegisterNewProtocol(name string, new NewProtocol) {
	dbg.Lvl3("Register new protocol:", name)
	ProtocolFactory.RegisterNewProtocol(name, new)
}

// RegisterProtocolConstructor is w wrapper around
// protocolFactory.RegisterProtocolConstructor
func RegisterProtocolConstructor(name string, cons ProtocolConstructor) {
	ProtocolFactory.RegisterProtocolConstructor(name, cons)
}

// Instantiate takes the name of the protocol and returns a fresh instance out
// of it.
func (pf *protocolFactory) Instantiate(name string, node *Node) (ProtocolInstance, error) {
	id, ok := pf.translations[name]
	if !ok {
		return nil, errors.New("Instantiate() No registered constructor at this name <" + name + ">")
	}
	return pf.InstantiateByID(id, node)
}

// InstantiateByID is equivalent of Instantiate using the id instead.
func (pf *protocolFactory) InstantiateByID(id ProtocolID, node *Node) (ProtocolInstance, error) {
	cons, ok := pf.constructors[id]
	if !ok {
		dbg.Lvl1("ProtocolFactory:", pf.translations)
		return nil, errors.New("Instantiate() No registered constructor at this id <" + pf.Name(id) + ">" + id.String())
	}
	return cons.NewProtocol(node)
}

// ProtocolID returns the ProtocolID out of the name
func (pf *protocolFactory) ProtocolID(name string) ProtocolID {
	id, ok := pf.translations[name]
	if !ok {
		return EmptyProtocolID
	}
	return id
}

func (pf *protocolFactory) Name(id ProtocolID) string {
	name := pf.reverse[id]
	return name
}

func (pf *protocolFactory) register(name string, cons ProtocolConstructor) {
	id := ProtocolID(uuid.NewV5(uuid.NamespaceURL, name))
	if _, ok := pf.constructors[id]; ok {
		dbg.Lvl2("Already have a protocol registered at the same name" + name)
	}
	pf.constructors[id] = cons
	pf.translations[name] = id
	pf.reverse[id] = name
}

// a defaultFactory is a factory that takes a NewProtocol and instantiates it
// anytime it is requested without any additional control.
type defaultConstructor struct {
	constructor NewProtocol
}

// implements the ProtocolConstructor  interface
func (df *defaultConstructor) NewProtocol(n *Node) (ProtocolInstance, error) {
	return df.constructor(n)
}

// ServiceID is an identifier for Service. Use a ServiceID to communicate to a
// service through its external API.
type ServiceID uuid.UUID

// String returns the string version of this ID
func (s *ServiceID) String() string {
	return uuid.UUID(*s).String()
}

// Equal returns true if both IDs are equal
func (s *ServiceID) Equal(s2 ServiceID) bool {
	return uuid.Equal(uuid.UUID(*s), uuid.UUID(s2))
}

// NilServiceID is the empty ID
var NilServiceID = ServiceID(uuid.Nil)

// Type of a function that is used to instantiate a given Service
// A service is initialized with a Host (to send messages to someone), the
// overlay (to register a Tree + EntityList + start new node), and a path where
// it can finds / write everything it needs
type NewServiceFunc func(h *Host, o *Overlay, path string) Service

// A serviceFactory is used to register a NewServiceFunc
type serviceFactory struct {
	cons map[ServiceID]NewServiceFunc
	// translations between name of a Service and its ServiceID. Used to register a
	// Service using a name.
	translations map[string]ServiceID
	// Inverse mapping of ServiceId => string
	inverseTr map[ServiceID]string
}

// the global service factory
var ServiceFactory = serviceFactory{
	cons:         make(map[ServiceID]NewServiceFunc),
	translations: make(map[string]ServiceID),
	inverseTr:    make(map[ServiceID]string),
}

// RegisterByName takes an name, creates a ServiceID out of it and store the
// mapping and the creation function.
func (s *serviceFactory) Register(name string, fn NewServiceFunc) {
	id := ServiceID(uuid.NewV5(uuid.NamespaceURL, name))
	if _, ok := s.cons[id]; ok {
		// called at init time so better panic than to continue
		dbg.Lvl1("RegisterService():", name)
	}
	s.cons[id] = fn
	s.translations[name] = id
	s.inverseTr[id] = name
}

// RegisterNewService is a wrapper around service factory
func RegisterNewService(name string, fn NewServiceFunc) {
	ServiceFactory.Register(name, fn)
}

// RegisteredServices returns all the services registered
func (s *serviceFactory) registeredServicesID() []ServiceID {
	var ids = make([]ServiceID, 0, len(s.cons))
	for id := range s.cons {
		ids = append(ids, id)
	}
	return ids
}

// RegisteredServicesByName returns all the names of the services registered
func (s *serviceFactory) RegisteredServicesName() []string {
	var names = make([]string, 0, len(s.translations))
	for n := range s.translations {
		names = append(names, n)
	}
	return names
}

// ServiceID returns the ServiceID out of the name of the service
func (s *serviceFactory) ServiceID(name string) ServiceID {
	var id ServiceID
	var ok bool
	if id, ok = s.translations[name]; !ok {
		return NilServiceID
	}
	return id
}

// Name returns the Name out of the ID
func (s *serviceFactory) Name(id ServiceID) string {
	var name string
	var ok bool
	if name, ok = s.inverseTr[id]; !ok {
		return ""
	}
	return name
}

// start launches a new service
func (s *serviceFactory) start(name string, host *Host, o *Overlay, path string) (Service, error) {
	var id ServiceID
	var ok bool
	if id, ok = s.translations[name]; !ok {
		return nil, errors.New("No Service for this name: " + name)
	}
	var fn NewServiceFunc
	if fn, ok = s.cons[id]; !ok {
		return nil, errors.New("No Service for this id:" + fmt.Sprintf("%v", id))
	}
	return fn(host, o, path), nil
}

// serviceStore is the place where all instantiated services are stored
// It gives access to :  all the currently running services and is handling the
// configuration path for them
type serviceStore struct {
	// the actual services
	services map[ServiceID]Service
	// the config paths
	paths map[ServiceID]string
}

const configFolder = "config"

// newServiceStore will create a serviceStore out of all the registered Service
// it creates the path for the config folder of each service. basically
// ```configFolder / *nameOfService*```
func newServiceStore(h *Host, o *Overlay) *serviceStore {
	// check if we have a config folder
	if err := os.MkdirAll(configFolder, 0777); err != nil {
		_, ok := err.(*os.PathError)
		if !ok {
			// we cannot continue from here
			panic(err)
		}
	}
	services := make(map[ServiceID]Service)
	configs := make(map[ServiceID]string)
	ids := ServiceFactory.registeredServicesID()
	for _, id := range ids {
		name := ServiceFactory.Name(id)
		pwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		configName := path.Join(pwd, configFolder, name)
		if err := os.MkdirAll(configName, 0666); err != nil {
			dbg.Error("Service", name, "Might not work properly: error setting up its config directory(", configName, "):", err)
		}
		s, err := ServiceFactory.start(name, h, o, configName)
		if err != nil {
			dbg.Error("Trying to instantiate service:", err)
		}
		dbg.Lvl2("Started Service", name, " (config in", configName, ")")
		services[id] = s
		configs[id] = configName
		// also register to the ProtocolFactory
		ProtocolFactory.RegisterProtocolConstructor(name, s)
	}
	return &serviceStore{services, configs}
}

// TODO
func (s *serviceStore) AvailableServices() []string {
	panic("not implemented")
}

// TODO
func (s *serviceStore) Service(name string) Service {
	return s.serviceByString(name)
}

// TODO
func (s *serviceStore) serviceByString(name string) Service {
	panic("Not implemented")
}

func (s *serviceStore) serviceByID(id ServiceID) Service {
	var serv Service
	var ok bool
	if serv, ok = s.services[id]; !ok {
		return nil
	}
	return serv
}

// A Request is a generic packet to represent any kind of request a Service is
// ready to process. It is simply a JSON packet containing two fields:
// * Service: a string representing the name of the service for whom the packet
// is intended for.
// * Data: contains all the information of the request
type Request struct {
	// Name of the service to direct this request to
	Service ServiceID `json:"service_id"`
	// Type is the type of the underlying request
	Type string `json:"type"`
	// Data containing all the information in the request
	Data *json.RawMessage `json:"data"`
}

// RequestType is the type that registered by the network library
var RequestType = network.RegisterMessageType(Request{})
