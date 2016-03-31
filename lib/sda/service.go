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
package sda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

// Service is a generic interface to define any type of services.
type Service interface {
	// InstantiateProtocol is called by sda when it receives a packet for an
	// non-existing Node (first contact). The service should provide the
	// ProtocolInstance with every information it might need.
	InstantiateProtocol(*Node) (ProtocolInstance, error)

	// ProcessRequest is the function that will be called when a external client
	// using the CLI will contact this service with a request packet.
	// Each request has a field ServiceID, so each time the Host (dispatcher)
	// receives a request, it looks whether it knows the Service it is for and
	// then dispatch it through ProcessRequest.
	ProcessRequest(*network.Entity, *Request)
}

// ID of a service (goes into the direction of having separate ID for each kind
// of object SDA use)
type ServiceID uuid.UUID

var NilServiceID = ServiceID(uuid.Nil)

// namespace for uuid generation
const namespace = "sda://serviceid/"

// Type of a function that is used to instantiate a given Service
// A service is initialized with a Host and a path where it can finds / write
// everything it needs
type NewServiceFunc func(h *Host, path string) Service

// A serviceFactory is responsible for creating all the services that registered
// to it.
type serviceFactory map[ServiceID]NewServiceFunc

// the global service factory
var ServiceFactory serviceFactory = make(serviceFactory)

// translations between name of a Service and its ServiceID. Used to register a
// Service using a name.
var translations map[string]ServiceID = make(map[string]ServiceID)

// Inverse mapping of ServiceId => string
var inverseTr map[ServiceID]string = make(map[ServiceID]string)

// Register takes an ID and a NewServiceFunc and store that service
func (s *serviceFactory) Register(id ServiceID, fn NewServiceFunc) {
	(*s)[id] = fn
}

// RegisterByName takes an name, creates a ServiceID out of it and store the
// mapping and the creation function.
func (s *serviceFactory) RegisterByName(name string, fn NewServiceFunc) {
	id := ServiceID(uuid.NewV5(uuid.NamespaceURL, namespace+name))
	s.Register(id, fn)
	translations[name] = id
	inverseTr[id] = name
}

// RegisteredServices returns all the services registered
func (s *serviceFactory) RegisteredServices() []ServiceID {
	var ids = make([]ServiceID, 0, len(*s))
	for id := range *s {
		ids = append(ids, id)
	}
	return ids
}

// RegisteredServicesByName returns all the names of the services registered
func (s *serviceFactory) RegisteredServicesByName() []string {
	var names = make([]string, 0, len(translations))
	for n := range translations {
		names = append(names, n)
	}
	return names
}

// ServiceID returns the ServiceID out of the name of the service
func (s *serviceFactory) ServiceID(name string) ServiceID {
	var id ServiceID
	var ok bool
	if id, ok = translations[name]; !ok {
		return NilServiceID
	}
	return id
}

// Name returns the Name out of the ID
func (s *serviceFactory) Name(id ServiceID) string {
	var name string
	var ok bool
	if name, ok = inverseTr[id]; !ok {
		return ""
	}
	return name
}

// Start looks if the service is registered and instantiate the service
// Returns an error if the service is not registered
func (s *serviceFactory) Start(id ServiceID, host *Host, path string) (Service, error) {
	var ok bool
	var fn NewServiceFunc
	if fn, ok = (*s)[id]; !ok {
		return nil, errors.New("No Service for this id:" + fmt.Sprintf("%v", id))
	}
	return fn(host, path), nil
}

// StartByName is equivalent of Start but works with the name.
func (s *serviceFactory) StartByName(name string, host *Host, path string) (Service, error) {
	var id ServiceID
	var ok bool
	if id, ok = translations[name]; !ok {
		return nil, errors.New("No Service for this name: " + name)
	}
	return s.Start(id, host, path)
}

// A Request is a generic packet to represent any kind of request a Service is
// ready to process. It is simply a JSON packet containing two fields:
// * Service: a string representing the name of the service for whom the packet
// is intended for.
// * Data: contains all the information of the request
type Request struct {
	// Name of the service to direct this request to
	Service ServiceID
	// Type is the type of the underlying request
	Type string
	// Data containing all the information in the request
	Data *json.RawMessage
}
