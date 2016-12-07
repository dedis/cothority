package sda

import (
	"errors"
	"reflect"

	"strings"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
)

// ServiceProcessor allows for an easy integration of external messages
// into the Services. You have to embed it into your Service-struct,
// then it will offer a 'RegisterMessage'-method that takes a message of type
// 	func ReceiveMsg(si *network.ServerIdentity, msg *anyMessageType)(error, *replyMsg)
// where 'ReceiveMsg' is any name and 'anyMessageType' will be registered
// with the network. Once 'anyMessageType' is received by the service,
// the function 'ReceiveMsg' should return an error and any 'replyMsg' it
// wants to send.
type ServiceProcessor struct {
	handlers map[string]serviceHandler
	*Context
}

type serviceHandler struct {
	handler interface{}
	msgType reflect.Type
}

// NewServiceProcessor initializes your ServiceProcessor.
func NewServiceProcessor(c *Context) *ServiceProcessor {
	return &ServiceProcessor{
		handlers: make(map[string]serviceHandler),
		Context:  c,
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
//
// XXX Name should be changed but need to change also in dedis/cosi
func (p *ServiceProcessor) RegisterMessage(f interface{}) error {
	ft := reflect.TypeOf(f)
	// Check that we have the correct channel-type.
	if ft.Kind() != reflect.Func {
		return errors.New("Input is not function")
	}
	if ft.NumIn() != 1 {
		return errors.New("Need one argument: *struct")
	}
	cr := ft.In(0)
	if cr.Kind() != reflect.Ptr {
		return errors.New("Second argument must be a *pointer* to struct")
	}
	if cr.Elem().Kind() != reflect.Struct {
		return errors.New("Second argument must be a pointer to *struct*")
	}
	if ft.NumOut() != 2 {
		return errors.New("Need 2 return values: network.Body and ClientError")
	}
	if ft.Out(0) != reflect.TypeOf((*network.Body)(nil)).Elem() {
		return errors.New("1st return value has to be: network.Body, but is: " +
			ft.Out(0).String())
	}
	if ft.Out(1) != reflect.TypeOf((*ClientError)(nil)).Elem() {
		return errors.New("2nd return value has to be: ClientError, but is: " +
			ft.Out(1).String())
	}
	// Automatic registration of the message to the network library.
	log.Lvl4("Registering handler", cr.String())
	pm := strings.Split(cr.Elem().String(), ".")[1]
	p.handlers[pm] = serviceHandler{f, cr.Elem()}
	p.conode.websocket.RegisterMessageHandler(ServiceFactory.Name(p.servID), pm)
	return nil
}

// RegisterMessages takes a vararg of messages to register and returns
// the first error encountered or nil if everything was OK.
func (p *ServiceProcessor) RegisterMessages(procs ...interface{}) error {
	for _, pr := range procs {
		if err := p.RegisterMessage(pr); err != nil {
			return err
		}
	}
	return nil
}

// Process implements the Processor interface and dispatches ClientRequest messages.
func (p *ServiceProcessor) Process(packet *network.Packet) {
	log.Panic("Cannot handle message.")
}

// NewProtocol is a stub for services that don't want to intervene in the
// protocol-handling.
func (i *ServiceProcessor) NewProtocol(tn *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	return nil, nil
}

// ProcessClientRequest takes a request from a client, calculates the reply
// and sends it back.
func (p *ServiceProcessor) ProcessClientRequest(path string, buf []byte) ([]byte, ClientError) {
	mh, ok := p.handlers[path]
	reply, cerr := func() (interface{}, ClientError) {
		if !ok {
			return nil, NewClientErrorCode(WebSocketErrorPathNotFound, "")
		} else {
			msg := reflect.New(mh.msgType).Interface()
			err := protobuf.Decode(buf, msg)
			if err != nil {
				return nil, NewClientErrorCode(WebSocketErrorProtobufDecode, "")
			}

			to := reflect.TypeOf(mh.handler).In(0)
			f := reflect.ValueOf(mh.handler)

			arg := reflect.New(to.Elem())
			arg.Elem().Set(reflect.ValueOf(msg).Elem())
			ret := f.Call([]reflect.Value{arg})

			cerr := ret[1].Interface()

			if cerr != nil {
				return nil, cerr.(ClientError)
			} else {
				return ret[0].Interface(), nil
			}
		}
	}()
	if cerr != nil {
		return nil, cerr
	}
	buf, err := protobuf.Encode(reply)
	if err != nil {
		log.Error(buf)
		return nil, NewClientErrorCode(WebSocketErrorProtobufEncode, "")
	}
	return buf, nil
}
