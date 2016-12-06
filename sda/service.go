package sda

import (
	"errors"
	"fmt"
	"os"
	"path"

	"strings"

	"sync"

	"reflect"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
	"golang.org/x/net/websocket"
)

// Service is a generic interface to define any type of services.
// A Service has multiple roles:
// * Processing sda-external client requests with ProcessClientRequests
// * Handling sda-external information to ProtocolInstances created with
//  	NewProtocol
// * Handling any kind of messages between Services between different hosts with
//   	the Processor interface
type Service interface {
	NewProtocol(*TreeNodeInstance, *GenericConfig) (ProtocolInstance, error)
	// ProcessRequest is called when a external client will connect through
	// the websocket-port to this service. It returns a message that will be
	// sent back to the client.
	ProcessClientRequest(handler string, msg []byte) (reply []byte, errCode int)
	// Processor makes a Service being able to handle any kind of packets
	// directly from the network. It is used for inter service communications,
	// which are mostly single packets with no or little interactions needed. If
	// a complex logic is used for these messages, it's best to put that logic
	// into a ProtocolInstance that the Service will launch, since there's nicer
	// utilities for ProtocolInstance.
	network.Processor
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
// overlay (to register a Tree + Roster + start new node), and a path where
// it can finds / write everything it needs
type NewServiceFunc func(c *Context, path string) Service

// GenericConfig is a config that can withhold any type of specific configs for
// protocols. It is passed down to the service NewProtocol function.
type GenericConfig struct {
	Type uuid.UUID
}

// GenericConfigID is the ID used by the network library for sending / receiving
// GenericCOnfig
var GenericConfigID = network.RegisterPacketType(GenericConfig{})

// A serviceFactory is used to register a NewServiceFunc
type serviceFactory struct {
	constructors []serviceEntry
	mutex        sync.RWMutex
}

// A serviceEntry holds all references to a service
type serviceEntry struct {
	constructor NewServiceFunc
	serviceID   ServiceID
	name        string
}

// ServiceFactory is the global service factory to instantiate Services
var ServiceFactory = serviceFactory{
	constructors: []serviceEntry{},
}

// RegisterByName takes a name, creates a ServiceID out of it and stores the
// mapping and the creation function.
func (s *serviceFactory) Register(name string, fn NewServiceFunc) error {
	if s.ServiceID(name) != NilServiceID {
		return fmt.Errorf("service %s already registered", name)
	}
	id := ServiceID(uuid.NewV5(uuid.NamespaceURL, name))
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.constructors = append(s.constructors, serviceEntry{
		constructor: fn,
		serviceID:   id,
		name:        name,
	})
	return nil
}

// Unregister - mainly for tests
func (s *serviceFactory) Unregister(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	index := -1
	for i, c := range s.constructors {
		if c.name == name {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.New("Didn't find service " + name)
	}
	s.constructors = append(s.constructors[:index], s.constructors[index+1:]...)
	return nil
}

// RegisterNewService is a wrapper around service factory
func RegisterNewService(name string, fn NewServiceFunc) error {
	return ServiceFactory.Register(name, fn)
}

// UnregisterService removes a service from the global pool.
func UnregisterService(name string) error {
	return ServiceFactory.Unregister(name)
}

// RegisteredServices returns all the services registered
func (s *serviceFactory) registeredServiceIDs() []ServiceID {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var ids = make([]ServiceID, 0, len(s.constructors))
	for _, c := range s.constructors {
		ids = append(ids, c.serviceID)
	}
	return ids
}

// RegisteredServicesByName returns all the names of the services registered
func (s *serviceFactory) RegisteredServiceNames() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var names = make([]string, 0, len(s.constructors))
	for _, n := range s.constructors {
		names = append(names, n.name)
	}
	return names
}

// ServiceID returns the ServiceID out of the name of the service
func (s *serviceFactory) ServiceID(name string) ServiceID {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, c := range s.constructors {
		if name == c.name {
			return c.serviceID
		}
	}
	return NilServiceID
}

// Name returns the Name out of the ID
func (s *serviceFactory) Name(id ServiceID) string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, c := range s.constructors {
		if id == c.serviceID {
			return c.name
		}
	}
	return ""
}

// start launches a new service
func (s *serviceFactory) start(name string, con *Context, path string) (Service, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, c := range s.constructors {
		if name == c.name {
			return c.constructor(con, path), nil
		}
	}
	return nil, errors.New("Didn't find service " + name)
}

// serviceManager is the place where all instantiated services are stored
// It gives access to: all the currently running services and is handling the
// configuration path for them
type serviceManager struct {
	// the actual services
	services map[ServiceID]Service
	// the config paths
	paths map[ServiceID]string
	// the sda host
	conode *Conode
	// the dispatcher can take registration of Processors
	network.Dispatcher
}

const configFolder = "config"

// newServiceStore will create a serviceStore out of all the registered Service
// it creates the path for the config folder of each service. basically
// ```configFolder / *nameOfService*```
func newServiceManager(c *Conode, o *Overlay) *serviceManager {
	// check if we have a config folder
	if err := os.MkdirAll(configFolder, 0770); err != nil {
		_, ok := err.(*os.PathError)
		if !ok {
			// we cannot continue from here
			log.Panic(err)
		}
	}
	services := make(map[ServiceID]Service)
	configs := make(map[ServiceID]string)
	s := &serviceManager{services, configs, c, network.NewRoutineDispatcher()}
	ids := ServiceFactory.registeredServiceIDs()
	for _, id := range ids {
		name := ServiceFactory.Name(id)
		log.Lvl3("Starting service", name)
		pwd, err := os.Getwd()
		if err != nil {
			log.Panic(err)
		}
		configName := path.Join(pwd, configFolder, name)
		if err := os.MkdirAll(configName, 0770); err != nil {
			log.Error("Service", name, "Might not work properly: error setting up its config directory(", configName, "):", err)
		}
		cont := newContext(c, o, id, s)
		s, err := ServiceFactory.start(name, cont, configName)
		if err != nil {
			log.Error("Trying to instantiate service:", err)
		}
		log.Lvl3("Started Service", name, " (config in", configName, ")")
		services[id] = s
		configs[id] = configName
		c.websocket.RegisterService(name, s)
	}
	log.Lvl3(c.Address(), "instantiated all services")
	return s
}

// Process implements the Processor interface: service manager will relay
// messages to the right Service.
func (s *serviceManager) Process(data *network.Packet) {
	// will launch a go routine for that message
	s.Dispatch(data)
}

// RegisterProcessor the processor to the service manager and tells the host to dispatch
// this message to the service manager. The service manager will then dispatch
// the message in a go routine. XXX This is needed because we need to have
// messages for service dispatched in asynchronously regarding the protocols.
// This behavior with go routine is fine for the moment but for better
// performance / memory / resilience, it may be changed to a real queuing
// system later.
func (s *serviceManager) RegisterProcessor(p network.Processor, msgType network.PacketTypeID) {
	// delegate message to host so the host will pass the message to ourself
	s.conode.RegisterProcessor(s, msgType)
	// handle the message ourselves (will be launched in a go routine)
	s.Dispatcher.RegisterProcessor(p, msgType)
}

func (s *serviceManager) RegisterProcessorFunc(msgType network.PacketTypeID, fn func(*network.Packet)) {
	// delegate message to host so the host will pass the message to ourself
	s.conode.RegisterProcessor(s, msgType)
	// handle the message ourselves (will be launched in a go routine)
	s.Dispatcher.RegisterProcessorFunc(msgType, fn)

}

// AvailableServices returns a list of all services available to the serviceManager.
// If no services are instantiated, it returns an empty list.
func (s *serviceManager) AvailableServices() (ret []string) {
	for id := range s.services {
		ret = append(ret, ServiceFactory.Name(id))
	}
	return
}

// Service returns the Service implementation being registered to this name or
// nil if no service by this name is available.
func (s *serviceManager) Service(name string) Service {
	id := ServiceFactory.ServiceID(name)
	if id == NilServiceID {
		return nil
	}
	return s.services[id]
}

func (s *serviceManager) serviceByID(id ServiceID) (Service, bool) {
	var serv Service
	var ok bool
	if serv, ok = s.services[id]; !ok {
		return nil, false
	}
	return serv, true
}

// Client is a struct used to communicate with a remote Service running on a
// sda.Conode
type Client struct {
	service string
}

// NewClient returns a client using the service s. It uses TCP communication by
// default
func NewClient(s string) *Client {
	return &Client{
		service: s,
	}
}

// Send will marshal the message into a ClientRequest message and send it.
func (c *Client) Send(dst *network.ServerIdentity, path string, buf []byte) ([]byte, error) {
	// Open connection to service.
	url, err := getWebHost(dst)
	if err != nil {
		return nil, err
	}
	log.Lvlf4("Sending %x to %s/%s/%s", buf, url, c.service, path)
	ws, err := websocket.Dial(fmt.Sprintf("ws://%s/%s/%s", url, c.service, path),
		"", "http://localhost/")
	if err != nil {
		return nil, err
	}
	if err = websocket.Message.Send(ws, buf); err != nil {
		return nil, err
	}
	var rcv []byte
	if err = websocket.Message.Receive(ws, &rcv); err != nil {
		return nil, err
	}
	log.Lvlf4("Received %x", rcv)
	return rcv, nil
}

// SendReflect wraps protobuf.(En|De)code to the Client.Send-function.
func (c *Client) SendProtobuf(dst *network.ServerIdentity, msg interface{}, ret interface{}) error {
	buf, err := protobuf.Encode(msg)
	if err != nil {
		return err
	}
	path := strings.Split(reflect.TypeOf(msg).String(), ".")[1]
	buf, err = c.Send(dst, path, buf)
	if err != nil {
		return err
	}
	if ret != nil {
		return protobuf.Decode(buf, ret)
	}
	return nil
}

// SendToAll sends a message to all ServerIdentities of the Roster and returns
// all errors encountered concatenated together as a string.
func (c *Client) SendToAll(dst *Roster, path string, buf []byte) ([][]byte, error) {
	msgs := make([][]byte, len(dst.List))
	var errstrs []string
	for i, e := range dst.List {
		var err error
		msgs[i], err = c.Send(e, path, buf)
		if err != nil {
			errstrs = append(errstrs, fmt.Sprint(e.String(), err.Error()))
		}
	}
	var err error
	if len(errstrs) > 0 {
		err = errors.New(strings.Join(errstrs, "\n"))
	}
	return msgs, err
}
