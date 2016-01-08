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
	"reflect"
)

/// Encoding part ///

// Global Suite used by this network library.
// For the moment, this will stay,as our focus is not on having the possibility
// to use any suite we want (the decoding stuff is much harder then, because we
// dont want to send the suite in the wire).
// It will surely change in futur releases so we can permit this behavior.
var Suite = edwards.NewAES128SHA256Ed25519(false)

// Type of a packet
type Type int32

// ProtocolMessage is a type for any message that the user wants to send
type ProtocolMessage interface{}

// RegisterProtocolType register a custom "struct" / "packet" and get
// the allocated Type
// Pass simply your non-initialized struct
func RegisterProtocolType(msgType Type, msg ProtocolMessage) error {
	if _, typeRegistered := typeRegistry[msgType]; typeRegistered {
		return errors.New("Type was already registered")
	}
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	t := val.Type()
	typeRegistry[msgType] = t
	invTypeRegistry[t] = msgType

	return nil
}

func TypeFromData(msg ProtocolMessage) Type {
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()
	intType, ok := invTypeRegistry[typ]
	if !ok {
		return DefaultType
	}
	return intType
}

// Give a default constructors for protobuf out of this suite
func DefaultConstructors(suite abstract.Suite) protobuf.Constructors {
	constructors := make(protobuf.Constructors)
	var point abstract.Point
	var secret abstract.Secret
	constructors[reflect.TypeOf(&point).Elem()] = func() interface{} { return suite.Point() }
	constructors[reflect.TypeOf(&secret).Elem()] = func() interface{} { return suite.Secret() }
	return constructors
}

// ApplicationMessage is the container for any ProtocolMessage
type ApplicationMessage struct {
	// From field can be set by the receivinf connection itself, no need to
	// acutally transmit the value
	//From string
	// What kind of msg do we have
	MsgType Type
	// The underlying message
	Msg ProtocolMessage

	// which constructors are used
	Constructors protobuf.Constructors
	// possible error during unmarshaling so that upper layer can know it
	err error
	// Same for the origin of the message
	From string
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

var typeRegistry = make(map[Type]reflect.Type)
var invTypeRegistry = make(map[reflect.Type]Type)

var globalOrder = binary.LittleEndian

// DefaultType is reserved by the network library. When you receive a message of
// DefaultType, it is generally because an error happenned, then you can call
// Error() on it.
var DefaultType Type = 0

// This is the default empty message that is returned in case something went
// wrong.
var EmptyApplicationMessage = ApplicationMessage{MsgType: DefaultType}

// When you don't need any speical constructors, you can give nil to NewTcpHost
// and it will use this empty constructors
var emptyConstructors protobuf.Constructors

func init() {
	emptyConstructors = make(protobuf.Constructors)
}

// String returns the underlying type in human format
func (t Type) String() string {
	ty, ok := typeRegistry[t]
	if !ok {
		return "unknown"
	}
	return ty.Name()
}

// MarshalregisteredType will marshal a struct with its respective type into a
// slice of bytes. That slice of bytes can be then decoded in
// UnmarshalRegisteredType.
func MarshalRegisteredType(data ProtocolMessage) ([]byte, error) {
	var msgType Type
	if msgType = TypeFromData(data); msgType == DefaultType {
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
func UnmarshalRegisteredType(buf []byte, constructors protobuf.Constructors) (Type, ProtocolMessage, error) {
	b := bytes.NewBuffer(buf)
	var t Type
	if err := binary.Read(b, globalOrder, &t); err != nil {
		return DefaultType, nil, err
	}
	var typ reflect.Type
	var ok bool
	if typ, ok = typeRegistry[t]; !ok {
		return DefaultType, nil, fmt.Errorf("Type %s not registered.", t.String())
	}
	ptrVal := reflect.New(typ)
	ptr := ptrVal.Interface()
	var err error
	if err = protobuf.DecodeWithConstructors(b.Bytes(), ptr, constructors); err != nil {
		return DefaultType, nil, err
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

// ConstructFrom takes a ProtocolMessage and then construct a
// ApplicationMessage from it. Error if the type is unknown
func NewApplicationMessage(obj ProtocolMessage) (*ApplicationMessage, error) {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("Send takes a pointer to the message, not a copy...")
	}
	val = val.Elem()
	t := val.Type()
	ty, ok := invTypeRegistry[t]
	if !ok {
		return &ApplicationMessage{}, errors.New(fmt.Sprintf("Packet to send is not known. Please register packet: %s\n", t.String()))
	}
	return &ApplicationMessage{
		MsgType: ty,
		Msg:     obj}, nil
}
