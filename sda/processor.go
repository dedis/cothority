package sda

import (
	"errors"
	"reflect"

	"gopkg.in/dedis/cothority.v0/lib/dbg"

	"strings"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
)

// Dispatcher is a struct that will dispatch message coming from the network to
// Processors. Each Processor that want to receive all messages of a specific
// type must register them self to the dispatcher using `RegisterProcessor()`.
// The network layer must call `Dispatch()` each time it received a message, so
// the dispatcher is able to dispatch correctly to the right Processor for
// further analysis.
// For the moment, only BlockingDispatcher is implemented but later, one can
// easily imagine to have a dispatcher having producer/consumers in go routine
// so the call is not blocking.
type Dispatcher interface {
	// RegisterProcessor is called by a Processor so it can receive any packet
	// of type msgType.
	// **NOTE** In the current version, if a subequent call RegisterProcessor
	// happens, for the same msgType, the latest Processor will be used; there
	// is no *copy* or *duplication* of messages.
	RegisterProcessor(p Processor, msgType network.MessageTypeID)
	// Dispatch will find the right processor to dispatch the packet to. The id
	// is the identity of the author / sender of the packet.
	// It can be called for example by the network layer.
	// If no processor is found for this message type, then the message is
	// dropped.
	Dispatch(id *network.ServerIdentity, packet *network.Packet)
}

// BlockingDispatcher is a Dispatcher that simply calls `p.Process()` on a
// processor p each time it receives a message with `Dispatch`. It does *not*
// launch a go routine, or put the message in a queue, etc.
// It can be re-used for more complex dispatcher.
type BlockingDispatcher struct {
	procs map[network.MessageTypeID]Processor
}

// NewBlockingDispatcher will return a freshly initialized BlockingDispatcher
func NewBlockingDispatcher() *BlockingDispatcher {
	return &BlockingDispatcher{
		procs: make(map[network.MessageTypeID]Processor),
	}
}

// RegisterProcessor save the given processor in the dispatcher.
func (d *BlockingDispatcher) RegisterProcessor(p Processor, msgType network.MessageTypeID) {
	d.procs[msgType] = p
}

// Dispatch will directly call the right processor's method Process. It's a
// blocking call if the Processor is blocking !
func (d *BlockingDispatcher) Dispatch(id *network.ServerIdentity, packet *network.Packet) {
	var p Processor
	if p = d.procs[packet.MsgType]; p == nil {
		dbg.Lvl2("Dispatcher received packet with no processor associated")
		return
	}
	p.Process(id, packet)
}

// Processor is an abstraction to represent any object that want to process
// packets. It is used in conjunction with Dispatcher.
type Processor interface {
	// Process takes a ServerIdentity as the sender identity and the message
	// sent.
	Process(id *network.ServerIdentity, packet *network.Packet)
}

// ServiceProcessor allows for an easy integration of external messages
// into the Services. You have to embed it into your Service-structer,
// then it will offer an 'RegisterMessage'-method that takes a message of type
// 	func ReceiveMsg(e *network.ServerIdentity, msg *anyMessageType)(error, *replyMsg)
// where 'ReceiveMsg' is any name and 'anyMessageType' will be registered
// with the network. Once 'anyMessageType' is received by the service,
// the function 'ReceiveMsg' should return an error and any 'replyMsg' it
// wants to send.
type ServiceProcessor struct {
	functions map[network.MessageTypeID]interface{}
	*Context
}

// NewServiceProcessor initializes your ServiceProcessor.
func NewServiceProcessor(c *Context) *ServiceProcessor {
	s := &ServiceProcessor{
		functions: make(map[network.MessageTypeID]interface{}),
		Context:   c,
	}
	// register the client messages
	c.host.RegisterProcessor(s, RequestID)
	// register the service messages
	c.host.RegisterProcessor(s, ServiceMessageID)
	return s
}

// RegisterMessage puts a new message in the message-handler
// XXX More comments are needed as it's not clear whether RegisterMessage waits
// for message for/from Clients or for/from Services.
func (p *ServiceProcessor) RegisterMessage(f interface{}) error {
	ft := reflect.TypeOf(f)
	// Check we have the correct channel-type
	if ft.Kind() != reflect.Func {
		return errors.New("Input is not function")
	}
	if ft.NumIn() != 2 {
		return errors.New("Need two arguments: *network.ServerIdentity and *struct")
	}
	if ft.In(0) != reflect.TypeOf(&network.ServerIdentity{}) {
		return errors.New("First argument must be *network.ServerIdentity")
	}
	cr1 := ft.In(1)
	if cr1.Kind() != reflect.Ptr {
		return errors.New("Second argument must be a *pointer* to struct")
	}
	if cr1.Elem().Kind() != reflect.Struct {
		return errors.New("Second argument must be a pointer to *struct*")
	}
	if ft.NumOut() != 2 {
		return errors.New("Need 2 return values: network.ProtocolMessage and error")
	}
	if ft.Out(0) != reflect.TypeOf((*network.Body)(nil)).Elem() {
		return errors.New("Need 2 return values: *network.ProtocolMessage* and error")
	}
	if ft.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return errors.New("Need 2 return values: network.ProtocolMessage and *error*")
	}
	// Automatic registration of the message to the network library.
	log.Lvl4("Registering handler", cr1.String())
	typ := network.RegisterMessageUUID(network.RTypeToMessageTypeID(
		cr1.Elem()),
		cr1.Elem())
	p.functions[typ] = f
	return nil
}

func (p *ServiceProcessor) Process(id *network.ServerIdentity, packet *network.Packet) {
	// check client type
	switch packet.MsgType {
	case RequestID:
		cr := packet.Msg.(ClientRequest)
		p.ProcessClientRequest(id, &cr)
	case ServiceMessageID:
		sm := packet.Msg.(InterServiceMessage)
		p.ProcessServiceMessage(id, &sm)
	}
}

// ProcessClientRequest takes a request from a client, calculates the reply
// and sends it back.
func (p *ServiceProcessor) ProcessClientRequest(e *network.ServerIdentity,
	cr *ClientRequest) {
	reply := p.GetReply(e, cr.Data)
	if err := p.SendRaw(e, reply); err != nil {
		log.Error(err)
	}
}

// ProcessServiceMessage is to implement the Service interface.
func (p *ServiceProcessor) ProcessServiceMessage(e *network.ServerIdentity,
	s *InterServiceMessage) {
	p.GetReply(e, s.Data)
}

// SendISM takes the message and sends it to the corresponding service
func (p *ServiceProcessor) SendISM(e *network.ServerIdentity, msg network.Body) error {
	sName := ServiceFactory.Name(p.Context.ServiceID())
	sm, err := CreateServiceMessage(sName, msg)
	if err != nil {
		return err
	}
	log.Lvl4("Raw-sending to", e)
	return p.SendRaw(e, sm)
}

// SendISMOthers sends an InterServiceMessage to all other services
func (p *ServiceProcessor) SendISMOthers(el *Roster, msg network.Body) error {
	var errStrs []string
	for _, e := range el.List {
		if !e.ID.Equal(p.Context.ServerIdentity().ID) {
			log.Lvl3("Sending to", e)
			err := p.SendISM(e, msg)
			if err != nil {
				errStrs = append(errStrs, err.Error())
			}
		}
	}
	var err error
	if len(errStrs) > 0 {
		err = errors.New(strings.Join(errStrs, "\n"))
	}
	return err
}

// GetReply takes a clientRequest and passes it to the corresponding
// handler-function.
func (p *ServiceProcessor) GetReply(e *network.ServerIdentity, d []byte) network.Body {
	mt, m, err := network.UnmarshalRegisteredType(d,
		network.DefaultConstructors(network.Suite))
	fu, ok := p.functions[mt]
	if !ok {
		return &StatusRet{"Didn't register message-handler: " + mt.String()}
	}

	if err != nil {
		return &StatusRet{err.Error()}
	}

	//to0 := reflect.TypeOf(fu).In(0)
	to1 := reflect.TypeOf(fu).In(1)
	f := reflect.ValueOf(fu)

	log.Lvl4("Dispatching to", e.Addresses)
	arg0 := reflect.New(reflect.TypeOf(network.ServerIdentity{}))
	arg0.Elem().Set(reflect.ValueOf(e).Elem())
	arg1 := reflect.New(to1.Elem())
	arg1.Elem().Set(reflect.ValueOf(m))

	ret := f.Call([]reflect.Value{arg0, arg1})

	errI := ret[1].Interface()

	if errI != nil {
		return &StatusRet{errI.(error).Error()}
	}

	reply := ret[0].Interface()
	if reply == nil {
		reply = StatusOK
	}
	return reply
}
