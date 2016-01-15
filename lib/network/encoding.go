package network

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
	"reflect"
)

/// Encoding part ///

// Suite used globally by this network library.
// For the moment, this will stay,as our focus is not on having the possibility
// to use any suite we want (the decoding stuff is much harder then, because we
// dont want to send the suite in the wire).
// It will surely change in futur releases so we can permit this behavior.
var Suite = edwards.NewAES128SHA256Ed25519(false)

// NetworkMessage is a type for any message that the user wants to send
type NetworkMessage interface{}

// When you don't need any special constructors, you can give nil to NewTcpHost
// and it will use this empty constructors
var emptyConstructors protobuf.Constructors

func init() {
	emptyConstructors = make(protobuf.Constructors)
}

// RegisterMessageType register a custom "struct" / "packet" and get
// the allocated Type
// Pass simply your non-initialized struct
func RegisterMessageType(msg NetworkMessage) uuid.UUID {
	// We add a star here because in TypeFromData we'll always have pointers,
	// so we're directly compatible with it
	url := "http://dedis.epfl.ch/protocolType/*" + reflect.TypeOf(msg).String()
	msgType := uuid.NewV3(uuid.NamespaceURL, url)
	if _, typeRegistered := typeRegistry[msgType]; typeRegistered {
		return msgType
	}
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	t := val.Type()
	typeRegistry[msgType] = t

	return msgType
}

// TypeFromData returns the corresponding uuid to the structure given. It
// returns 'DefaultType' upon error.
func TypeFromData(msg NetworkMessage) uuid.UUID {
	url := "http://dedis.epfl.ch/protocolType/" + reflect.TypeOf(msg).String()
	msgType := uuid.NewV3(uuid.NamespaceURL, url)
	_, ok := typeRegistry[msgType]
	if !ok {
		return ErrorType
	}
	return msgType
}

// DumpTypes is used for debugging - it prints out all known types
func DumpTypes() {
	for t, m := range typeRegistry {
		dbg.Print("Type", t, "has message", m)
	}
}

// DefaultConstructors gives a default constructor for protobuf out of the global suite
func DefaultConstructors(suite abstract.Suite) protobuf.Constructors {
	constructors := make(protobuf.Constructors)
	var point abstract.Point
	var secret abstract.Secret
	constructors[reflect.TypeOf(&point).Elem()] = func() interface{} { return suite.Point() }
	constructors[reflect.TypeOf(&secret).Elem()] = func() interface{} { return suite.Secret() }
	return constructors
}

// Error returns the error that has been encountered during the unmarshaling of
// this message.
func (am *ApplicationMessage) Error() error {
	return am.err
}

// workaround so we can set the error after creation of the application
// message...
func (am *ApplicationMessage) SetError(err error) {
	am.err = err
}

var typeRegistry = make(map[uuid.UUID]reflect.Type)

var globalOrder = binary.LittleEndian

// ErrorType is reserved by the network library. When you receive a message of
// ErrorType, it is generally because an error happened, then you can call
// Error() on it.
var ErrorType uuid.UUID = uuid.Nil

// This is the default empty message that is returned in case something went
// wrong.
var EmptyApplicationMessage = ApplicationMessage{MsgType: ErrorType}

// MarshalRegisteredType will marshal a struct with its respective type into a
// slice of bytes. That slice of bytes can be then decoded in
// UnmarshalRegisteredType.
func MarshalRegisteredType(data NetworkMessage) ([]byte, error) {
	var msgType uuid.UUID
	if msgType = TypeFromData(data); msgType == ErrorType {
		return nil, fmt.Errorf("Type %s Not registered to the network library.", msgType)
	}
	b := new(bytes.Buffer)
	if err := binary.Write(b, globalOrder, msgType); err != nil {
		return nil, err
	}
	var buf []byte
	var err error
	if buf, err = protobuf.Encode(data); err != nil {
		dbg.Error("Error for protobuf encoding:", err)
		return nil, err
	}
	_, err = b.Write(buf)
	return b.Bytes(), err
}

// UnmarshalRegisteredType returns the type, the data and an error trying to
// decode a message from a buffer. The type must be registered to the network
// library in order for it to be decodable.
func UnmarshalRegisteredType(buf []byte, constructors protobuf.Constructors) (uuid.UUID, NetworkMessage, error) {
	b := bytes.NewBuffer(buf)
	var t uuid.UUID
	if err := binary.Read(b, globalOrder, &t); err != nil {
		return ErrorType, nil, err
	}
	var typ reflect.Type
	var ok bool
	if typ, ok = typeRegistry[t]; !ok {
		return ErrorType, nil, fmt.Errorf("Type %s not registered.", t.String())
	}
	ptrVal := reflect.New(typ)
	ptr := ptrVal.Interface()
	var err error
	if err = protobuf.DecodeWithConstructors(b.Bytes(), ptr, constructors); err != nil {
		return ErrorType, nil, err
	}
	return t, ptrVal.Elem().Interface(), nil
}

// MarshalBinary the application message => to bytes
// Implements BinaryMarshaler interface so it will be used when sending with protobuf
func (am *ApplicationMessage) MarshalBinary() ([]byte, error) {
	return MarshalRegisteredType(am.Msg)
}

// UnmarshalBinary will decode the incoming bytes
// It checks if the underlying packet is self-decodable
// by using its UnmarshalBinary interface
// otherwise, use abstract.Encoding (suite) to decode
func (am *ApplicationMessage) UnmarshalBinary(buf []byte) error {
	t, msg, err := UnmarshalRegisteredType(buf, am.Constructors)
	am.MsgType = t
	am.Msg = msg
	return err
}

// ConstructFrom takes a NetworkMessage and then construct a
// ApplicationMessage from it. Error if the type is unknown
func newApplicationMessage(obj NetworkMessage) (*ApplicationMessage, error) {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("Send takes a pointer to the message, not a copy...")
	}
	ty := TypeFromData(obj)
	if ty == ErrorType {
		return &ApplicationMessage{}, errors.New(fmt.Sprintf("Packet to send is not known. Please register packet: %s\n",
			reflect.TypeOf(obj).String()))
	}
	return &ApplicationMessage{
		MsgType: ty,
		Msg:     obj}, nil
}
