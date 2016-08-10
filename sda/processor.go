package sda

import (
	"errors"
	"reflect"

	"strings"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
)

// Dispatcher is an interface whose sole role is to distribute messages to the
// right Processor. No processing is done,i.e. no looking at packet content.
// There are many ways to distribute messages: for the moment, only
// BlockingDispatcher is implemented, which is a blocking dispatcher.
// Later, one can easily imagine to have a dispatcher with one worker in a
// goroutine or a fully fledged producer/consumers pattern in go routines.
// Each Processor that wants to receive all messages of a specific
// type must register itself to the dispatcher using `RegisterProcessor()`.
// The network layer must call `Dispatch()` each time it receives a message, so
// the dispatcher is able to dispatch correctly to the right Processor for
// further analysis.
type Dispatcher interface {
	// RegisterProcessor is called by a Processor so it can receive all packets
	// of type msgType. If given multiple msgType, the same processor will be
	// called for each of all the msgType given.
	// **NOTE** In the current version, if a subequent call to RegisterProcessor
	// happens, for the same msgType, the latest Processor will be used; there
	// is no *copy* or *duplication* of messages.
	RegisterProcessor(p Processor, msgType ...network.PacketTypeID)
	// Dispatch will find the right processor to dispatch the packet to.
	// The id identifies the sender of the packet.
	// It can be called for example by the network layer.
	// If no processor is found for this message type, then an error is returned
	Dispatch(packet *network.Packet) error
}

// Processor is an abstraction to represent any object that want to process
// packets. It is used in conjunction with Dispatcher:
// A processor must register itself to a Dispatcher so the Dispatcher will
// dispatch every messages to the Processor asked for.
type Processor interface {
	// Process takes a received network.Packet.
	Process(packet *network.Packet)
}

// BlockingDispatcher is a Dispatcher that simply calls `p.Process()` on a
// processor p each time it receives a message with `Dispatch`. It does *not*
// launch a go routine, or put the message in a queue, etc.
// It can be re-used for more complex dispatcher.
type BlockingDispatcher struct {
	procs map[network.PacketTypeID]Processor
}

// NewBlockingDispatcher will return a freshly initialized BlockingDispatcher.
func NewBlockingDispatcher() *BlockingDispatcher {
	return &BlockingDispatcher{
		procs: make(map[network.PacketTypeID]Processor),
	}
}

// RegisterProcessor saves the given processor in the dispatcher.
func (d *BlockingDispatcher) RegisterProcessor(p Processor, msgType ...network.PacketTypeID) {
	for _, t := range msgType {
		d.procs[t] = p
	}
}

// Dispatch will directly call the right processor's method Process. It's a
// blocking call if the Processor is blocking !
func (d *BlockingDispatcher) Dispatch(packet *network.Packet) error {
	var p Processor
	if p = d.procs[packet.MsgType]; p == nil {
		return errors.New("No Processor attached to this message type.")
	}
	p.Process(packet)
	return nil
}

// RoutineDispatcher is a Dispatcher that will dispatch messages to Processor
// in a go routine. RoutineDispatcher creates one go routine per message it
// receives.
type RoutineDispatcher struct {
	*BlockingDispatcher
}

// NewRoutineDispatcher returns a fresh RoutineDispatcher
func NewRoutineDispatcher() *RoutineDispatcher {
	return &RoutineDispatcher{
		BlockingDispatcher: NewBlockingDispatcher(),
	}
}

// Dispatch implements the Dispatcher interface. It will give the packet to the
// right Processor in a go routine.
func (d *RoutineDispatcher) Dispatch(packet *network.Packet) error {
	var p = d.procs[packet.MsgType]
	if p == nil {
		return errors.New("No Processor attached to this message type.")
	}
	go p.Process(packet)
	return nil
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
	functions map[network.PacketTypeID]interface{}
	*Context
}

// NewServiceProcessor initializes your ServiceProcessor.
func NewServiceProcessor(c *Context) *ServiceProcessor {
	return &ServiceProcessor{
		functions: make(map[network.PacketTypeID]interface{}),
		Context:   c,
	}
}

// RegisterMessage will store the given handler that will be used by the service.
// f must be a function of the following form:
// func(sId *network.ServerIdentity, structPtr *MyMessageStruct)(network.Body, error)
//
// In other words:
// f must be a function that takes two arguments:
//  * network.ServerIdentity: from whom the message is coming from.
//  * Pointer to a struct: message that the service is ready to handle.
// f must have two return values:
//  * Pointer to a struct: message that the service has generated as a reply and
//  that will be sent to the requester (the sender).
//  * Error in any case there is an error.
// f can be used to treat internal service messages as well as external requests
// from clients.
// XXX Name should be changed but need to change also in dedis/cosi
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
		return errors.New("Need 2 return values: network.Body and error")
	}
	if ft.Out(0) != reflect.TypeOf((*network.Body)(nil)).Elem() {
		return errors.New("Need 2 return values: *network.Body* and error")
	}
	if ft.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return errors.New("Need 2 return values: network.Body and *error*")
	}
	// Automatic registration of the message to the network library.
	log.Lvl4("Registering handler", cr1.String())
	typ := network.RegisterPacketUUID(network.RTypeToPacketTypeID(
		cr1.Elem()),
		cr1.Elem())
	p.functions[typ] = f
	return nil
}

// Process implements the Processor interface and dispatches ClientRequest message
// and InterServiceMessage
func (p *ServiceProcessor) Process(packet *network.Packet) {
	p.GetReply(packet.ServerIdentity, packet.MsgType, packet.Msg)
}

// ProcessClientRequest takes a request from a client, calculates the reply
// and sends it back.
func (p *ServiceProcessor) ProcessClientRequest(e *network.ServerIdentity,
	cr *ClientRequest) {
	// unmarshal the inner message
	mt, m, err := network.UnmarshalRegisteredType(cr.Data,
		network.DefaultConstructors(network.Suite))
	if err != nil {
		log.Error("Err unmarshal client request:" + err.Error())
		return
	}
	reply := p.GetReply(e, mt, m)
	if err := p.SendRaw(e, reply); err != nil {
		log.Error(err)
	}
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

// GetReply takes msgType and a message. It dispatches the msg to the right
// function registered, then sends the responses to the sender.
func (p *ServiceProcessor) GetReply(e *network.ServerIdentity, mt network.PacketTypeID, m network.Body) network.Body {
	fu, ok := p.functions[mt]
	if !ok {
		return &StatusRet{"Didn't register message-handler: " + mt.String()}
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
