package skipchain

import (
	"errors"
	"reflect"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

type ServiceProcessor struct {
	functions map[network.MessageTypeID]interface{}
	sda.Context
}

func NewProcessor(c sda.Context) *ServiceProcessor {
	return &ServiceProcessor{
		functions: make(map[network.MessageTypeID]interface{}),
		Context:   c,
	}
}

// AddMessage puts a new message in the message-handler
func (p *ServiceProcessor) AddMessage(f interface{}) error {
	ft := reflect.TypeOf(f)
	// Check we have the correct channel-type
	if ft.Kind() != reflect.Func {
		return errors.New("Input is not function")
	}
	if ft.NumIn() != 2 {
		return errors.New("Need two arguments: *network.Entity and *struct")
	}
	if ft.In(0) != reflect.TypeOf(&network.Entity{}) {
		return errors.New("First argument must be *network.Entity")
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
	if ft.Out(0) != reflect.TypeOf((*network.ProtocolMessage)(nil)).Elem() {
		return errors.New("Need 2 return values: *network.ProtocolMessage* and error")
	}
	if ft.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return errors.New("Need 2 return values: network.ProtocolMessage and *error*")
	}
	// Automatic registration of the message to the network library.
	dbg.Lvl3("Registering handler", cr1.String())
	typ := network.RegisterMessageUUID(network.RTypeToMessageTypeID(
		cr1.Elem()),
		cr1.Elem())
	p.functions[typ] = f
	return nil
}

// ProcessClientRequest takes a request from a client, calculates the reply
// and sends it back.
func (p *ServiceProcessor) ProcessClientRequest(e *network.Entity,
	cr *sda.ClientRequest) {
	reply := p.GetReply(e, cr)

	if err := p.SendRaw(e, reply); err != nil {
		dbg.Error(err)
	}
}

// ProcessServiceMessage is to implement the Service interface.
func (p *ServiceProcessor) ProcessServiceMessage(e *network.Entity,
	s *sda.ServiceMessage) {
	cr := &sda.ClientRequest{
		Data: s.Data,
	}
	p.GetReply(e, cr)
}

// GetReply takes a clientRequest and passes it to the corresponding
// handler-function.
func (p *ServiceProcessor) GetReply(e *network.Entity, cr *sda.ClientRequest) network.ProtocolMessage {
	mt := cr.Type
	fu, ok := p.functions[mt]
	if !ok {
		return &sda.StatusRet{"Don't know message: " + mt.String()}
	}

	_, m, err := network.UnmarshalRegisteredType(cr.Data,
		network.DefaultConstructors(network.Suite))

	if err != nil {
		return &sda.StatusRet{err.Error()}
	}

	//to0 := reflect.TypeOf(fu).In(0)
	to1 := reflect.TypeOf(fu).In(1)
	f := reflect.ValueOf(fu)

	dbg.Lvl4("Dispatching to", e.Addresses)
	arg0 := reflect.New(reflect.TypeOf(network.Entity{}))
	arg0.Elem().Set(reflect.ValueOf(e).Elem())
	arg1 := reflect.New(to1.Elem())
	arg1.Elem().Set(reflect.ValueOf(m))

	ret := f.Call([]reflect.Value{arg0, arg1})

	errI := ret[1].Interface()

	if errI != nil {
		return &sda.StatusRet{errI.(error).Error()}
	}

	return ret[0].Interface()
}
