package sda

import (
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
)

func init() {
	network.RegisterMessageType(&StatusRet{})
}

// Service is a generic interface to define any type of services.
type Service interface {
	NewProtocol(*TreeNodeInstance, *GenericConfig) (ProtocolInstance, error)
	// ProcessRequest is the function that will be called when a external client
	// using the CLI will contact this service with a request packet.
	// Each request has a field ServiceID, so each time the Host (dispatcher)
	// receives a request, it looks whether it knows the Service it is for and
	// then dispatch it through ProcessRequest.
	ProcessClientRequest(*network.Entity, *ClientRequest)
	// ProcessServiceRequest takes a message from another Service
	ProcessServiceMessage(*network.Entity, *ServiceMessage)
}

// ServiceID is a type to represent a uuid for a Service
type ServiceID uuid.UUID

// String returns the string representation of this ServiceID
func (s *ServiceID) String() string {
	return uuid.UUID(*s).String()
}

// NilServiceID is the empty ServiceID
var NilServiceID = ServiceID(uuid.Nil)

// NewServiceFunc is the type of a function that is used to instantiate a given Service
// A service is initialized with a Host (to send messages to someone), the
// overlay (to register a Tree + EntityList + start new node), and a path where
// it can finds / write everything it needs
type NewServiceFunc func(c Context, path string) Service

// GenericConfig is a config that can withhold any type of specific configs for
// protocols. It is passed down to the service NewProtocol function.
type GenericConfig struct {
	Type uuid.UUID
	//Data network.ProtocolMessage
}

// GenericConfigID is the ID used by the network library for sending / receiving
// GenericCOnfig
var GenericConfigID = network.RegisterMessageType(GenericConfig{})

// A serviceFactory is used to register a NewServiceFunc
type serviceFactory struct {
	constructors map[ServiceID]NewServiceFunc
	// translations between name of a Service and its ServiceID. Used to register a
	// Service using a name.
	translations map[string]ServiceID
	// Inverse mapping of ServiceId => string
	inverseTr map[ServiceID]string
}

// ServiceFactory is the global service factory to instantiate Services
var ServiceFactory = serviceFactory{
	constructors: make(map[ServiceID]NewServiceFunc),
	translations: make(map[string]ServiceID),
	inverseTr:    make(map[ServiceID]string),
}

// RegisterByName takes an name, creates a ServiceID out of it and store the
// mapping and the creation function.
func (s *serviceFactory) Register(name string, fn NewServiceFunc) {
	id := ServiceID(uuid.NewV5(uuid.NamespaceURL, name))
	if _, ok := s.constructors[id]; ok {
		// called at init time so better panic than to continue
		dbg.Lvl1("RegisterService():", name)
	}
	s.constructors[id] = fn
	s.translations[name] = id
	s.inverseTr[id] = name
}

// RegisterNewService is a wrapper around service factory
func RegisterNewService(name string, fn NewServiceFunc) {
	ServiceFactory.Register(name, fn)
}

// RegisteredServices returns all the services registered
func (s *serviceFactory) registeredServicesID() []ServiceID {
	var ids = make([]ServiceID, 0, len(s.constructors))
	for id := range s.constructors {
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
	id, ok := s.translations[name]
	if !ok {
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
func (s *serviceFactory) start(name string, c Context, path string) (Service, error) {
	var id ServiceID
	var ok bool
	if id, ok = s.translations[name]; !ok {
		return nil, errors.New("No Service for this name: " + name)
	}
	var fn NewServiceFunc
	if fn, ok = s.constructors[id]; !ok {
		return nil, fmt.Errorf("No Service for this id: %+v", id)
	}
	serv := fn(c, path)
	dbg.Lvl2("Instantiated service", name)
	return serv, nil
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
			dbg.Panic(err)
		}
	}
	services := make(map[ServiceID]Service)
	configs := make(map[ServiceID]string)
	ids := ServiceFactory.registeredServicesID()
	for _, id := range ids {
		name := ServiceFactory.Name(id)
		pwd, err := os.Getwd()
		if err != nil {
			dbg.Panic(err)
		}
		configName := path.Join(pwd, configFolder, name)
		if err := os.MkdirAll(configName, 0666); err != nil {
			dbg.Error("Service", name, "Might not work properly: error setting up its config directory(", configName, "):", err)
		}
		c := newDefaultContext(h, o, id)
		s, err := ServiceFactory.start(name, c, configName)
		if err != nil {
			dbg.Error("Trying to instantiate service:", err)
		}
		dbg.Lvl2("Started Service", name, " (config in", configName, ")")
		services[id] = s
		configs[id] = configName
		// !! register to the ProtocolFactory !!
		//ProtocolFactory.registerService(id, s.NewProtocol)
	}
	dbg.Lvl3(h.workingAddress, "instantiated all services")
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

func (s *serviceStore) serviceByID(id ServiceID) (Service, bool) {
	var serv Service
	var ok bool
	if serv, ok = s.services[id]; !ok {
		return nil, false
	}
	return serv, true
}

// ClientRequest is a generic packet to represent any kind of request a Service is
// ready to process. It is simply a JSON packet containing two fields:
// * Service: a string representing the name of the service for whom the packet
// is intended for.
// * Data: contains all the information of the request
type ClientRequest struct {
	// Name of the service to direct this request to
	Service ServiceID
	// Data containing all the information in the request
	Data []byte
}

// RequestID is the type that registered by the network library
var RequestID = network.RegisterMessageType(ClientRequest{})

// CreateServiceRequest creates a Request message out of any message that is
// destined to a Service. XXX For the moment it uses protobuf, as it is already
// handling abstract.Secret/Public stuff that json can't do. Later we may want
// to think on how to change that.
func CreateServiceRequest(service string, r interface{}) (*ClientRequest, error) {
	sid := ServiceFactory.ServiceID(service)
	dbg.Lvl1("Name", service, " <-> ServiceID", sid.String())
	buff, err := network.MarshalRegisteredType(r)
	if err != nil {
		return nil, err
	}
	return &ClientRequest{
		Service: sid,
		Data:    buff,
	}, nil
}

// ServiceMessage is a generic struct that contains any data destined to a
// Service that has been created .. by a Service. => Intra-Service
// communications.
type ServiceMessage struct {
	// Service is the ID of the Service it's destined
	Service ServiceID
	// Data is the data encoded using protobuf for the moment.
	Data []byte
}

// ServiceMessageID is the ID of the ServiceMessage struct.
var ServiceMessageID = network.RegisterMessageType(ServiceMessage{})

// CreateServiceMessage takes a service name and some data and encodes the whole
// as a ServiceMessage.
func CreateServiceMessage(service string, r interface{}) (*ServiceMessage, error) {
	sid := ServiceFactory.ServiceID(service)
	buff, err := network.MarshalRegisteredType(r)
	if err != nil {
		return nil, err
	}
	return &ServiceMessage{
		Service: sid,
		Data:    buff,
	}, nil

}

/*
A simple client structure to be used when wanting to connect to services. It
holds the private and public key and allows to connect to a service through
the network.
The error-handling is done using the ErrorRet structure which can be returned
in place of the standard reply. The Client.Send method will catch that and return
 the appropriate error.
*/

// Client for a service
type Client struct {
	private abstract.Secret
	*network.Entity
	ServiceID ServiceID
}

// NewClient returns a random client using the service s
func NewClient(s string) *Client {
	kp := config.NewKeyPair(network.Suite)
	return &Client{
		Entity:    network.NewEntity(kp.Public, ""),
		private:   kp.Secret,
		ServiceID: ServiceFactory.ServiceID(s),
	}
}

// NetworkSend opens the connection to 'dst' and sends the message 'req'. The
// reply is returned, or an error if the timeout of 10 seconds is reached.
func (c *Client) Send(dst *network.Entity, msg network.ProtocolMessage) (*network.Message, error) {
	client := network.NewSecureTCPHost(c.private, c.Entity)

	// Connect to the root
	dbg.Lvl4("Opening connection to", dst)
	con, err := client.Open(dst)
	defer client.Close()
	if err != nil {
		return nil, err
	}

	m, err := network.NewNetworkMessage(msg)
	if err != nil {
		return nil, err
	}

	b, err := m.MarshalBinary()

	if err != nil {
		return nil, err
	}
	serviceReq := &ClientRequest{
		Service: c.ServiceID,
		Data:    b,
	}
	pchan := make(chan network.Message)
	go func() {
		// send the request
		dbg.Lvlf4("Sending request %+v", serviceReq)
		if err := con.Send(context.TODO(), serviceReq); err != nil {
			close(pchan)
			return
		}
		dbg.Lvl4("Waiting for the response")
		// wait for the response
		packet, err := con.Receive(context.TODO())
		if err != nil {
			close(pchan)
			return
		}
		pchan <- packet
	}()
	select {
	case response := <-pchan:
		dbg.Lvlf5("Response: %+v", response)
		// Catch an eventual error
		err := ErrMsg(&response, nil)
		if err != nil {
			return nil, err
		}
		return &response, nil
	case <-time.After(time.Second * 10):
		return &network.Message{}, errors.New("Timeout on sending message")
	}
}

// BinaryMarshaler can be used to store the client in a configuration-file
func (c *Client) BinaryMarshaler() ([]byte, error) {
	dbg.Fatal("Not yet implemented")
	return nil, nil
}

// BinaryUnmarshaler sets the different values from a byte-slice
func (c *Client) BinaryUnmarshaler(b []byte) error {
	dbg.Fatal("Not yet implemented")
	return nil
}

// StatusRet is used when a status is returned - mostly an error
type StatusRet struct {
	Status string
}

// ErrMsg converts a combined err and status-message to an error. It
// returns either the error, or the errormsg, if there is one.
func ErrMsg(em *network.Message, err error) error {
	if err != nil {
		return err
	}
	status, ok := em.Msg.(StatusRet)
	if !ok {
		return nil
	}
	statusStr := status.Status
	if statusStr != "" {
		return errors.New("Remote-error: " + statusStr)
	}
	return nil
}
