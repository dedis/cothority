package skipchain

import (
	"errors"
	"reflect"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

type Processor struct {
	functions map[network.MessageTypeID]interface{}
}

func NewProcessor() *Processor {
	return &Processor{
		functions: make(map[network.MessageTypeID]interface{}),
	}
}

// AddMessage puts a new message in the message-handler
func (p *Processor) AddMessage(f interface{}) error {
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
	if ft.Out(1) != reflect.TypeOf((*error)(nil)).Elem(){
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

func (p *Processor) ProcessClientRequest(e *network.Entity, cr *sda.ClientRequest) {
}

// ProcessServiceMessage is to implement the Service interface.
func (p *Processor) ProcessServiceMessage(e *network.Entity, s *sda.ServiceMessage) {
}

type testMsg struct {
	I int
}

func (p *Processor) GetReply(e *network.Entity, cr *sda.ClientRequest) network.ProtocolMessage {
	mt := cr.Type
	fu, ok := p.functions[mt]
	if !ok {
		return &sda.ErrorRet{errors.New("Don't know message: " + mt.String())}
	}
	_, m, err := network.UnmarshalRegisteredType(cr.Data,
		network.DefaultConstructors(network.Suite))
	if err != nil {
		return &sda.ErrorRet{err}
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
	if errI != nil{
		return &sda.ErrorRet{errI.(error)}
	}

	return ret[0].Interface()
}
