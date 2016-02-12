package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"sync"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
)

/// Encoding part ///

// Suite used globally by this network library.
// For the moment, this will stay,as our focus is not on having the possibility
// to use any suite we want (the decoding stuff is much harder then, because we
// don't want to send the suite in the wire).
// It will surely change in futur releases so we can permit this behavior.
var Suite = edwards.NewAES128SHA256Ed25519(false)

// ProtocolMessage is a type for any message that the user wants to send
type ProtocolMessage interface{}

// When you don't need any special constructors, you can give nil to NewTcpHost
// and it will use this empty constructors
var emptyConstructors protobuf.Constructors

// The basic url used for uuid
const UuidURL = "https://dedis.epfl.ch/"
const UuidURLProtocolType = UuidURL + "/protocolType/"

func init() {
	emptyConstructors = make(protobuf.Constructors)
}

// RegisterMessageType registers a custom "struct" / "packet" and get
// the allocated Type
// Pass simply your non-initialized struct
func RegisterMessageType(msg ProtocolMessage) uuid.UUID {
	msgType := TypeToUUID(msg)
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	t := val.Type()
	return RegisterMessageUUID(msgType, t)
}

// RegisterMessageUUID can be used if the uuid and the type is already known
// NOTE: be sure to only registers VALUE message and not POINTERS to message.
func RegisterMessageUUID(mt uuid.UUID, rt reflect.Type) uuid.UUID {
	registryLock.Lock()
	defer registryLock.Unlock()
	if _, typeRegistered := typeRegistry[mt]; typeRegistered {
		return mt
	}
	typeRegistry[mt] = rt

	return mt
}

// TypeFromData returns the corresponding uuid to the structure given. It
// returns 'DefaultType' upon error.
func TypeFromData(msg ProtocolMessage) uuid.UUID {
	registryLock.Lock()
	defer registryLock.Unlock()
	msgType := TypeToUUID(msg)
	_, ok := typeRegistry[msgType]
	if !ok {
		return ErrorType
	}
	return msgType
}

// TypeToUUID Converts MsgType to uuid
func TypeToUUID(msg ProtocolMessage) uuid.UUID {
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	url := UuidURLProtocolType + val.Type().String()
	u := uuid.NewV5(uuid.NamespaceURL, url)
	dbg.Lvl5("Reflecting", reflect.TypeOf(msg), "to", u)
	return u
}

// RTypeToUUID converts a reflect-type to a UUID
func RTypeToUUID(msg reflect.Type) uuid.UUID {
	url := UuidURLProtocolType + msg.String()
	return uuid.NewV5(uuid.NamespaceURL, url)
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
func (am *NetworkMessage) Error() error {
	return am.err
}

// SetError is workaround so we can set the error after creation of the
// application message
func (am *NetworkMessage) SetError(err error) {
	am.err = err
}

var typeRegistry = make(map[uuid.UUID]reflect.Type)
var registryLock = new(sync.Mutex)

var globalOrder = binary.LittleEndian

// ErrorType is reserved by the network library. When you receive a message of
// ErrorType, it is generally because an error happened, then you can call
// Error() on it.
var ErrorType uuid.UUID = uuid.Nil

// EmptyApplicationMessage is the default empty message that is returned in case
// something went wrong.
var EmptyApplicationMessage = NetworkMessage{MsgType: ErrorType}

// MarshalRegisteredType will marshal a struct with its respective type into a
// slice of bytes. That slice of bytes can be then decoded in
// UnmarshalRegisteredType.
func MarshalRegisteredType(data ProtocolMessage) ([]byte, error) {
	var msgType uuid.UUID
	if msgType = TypeFromData(data); msgType == ErrorType {
		return nil, fmt.Errorf("Type of message %s not registered to the network library.", reflect.TypeOf(data))
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
func UnmarshalRegisteredType(buf []byte, constructors protobuf.Constructors) (uuid.UUID, ProtocolMessage, error) {
	b := bytes.NewBuffer(buf)
	var t uuid.UUID
	if err := binary.Read(b, globalOrder, &t); err != nil {
		return ErrorType, nil, err
	}
	var typ reflect.Type
	var ok bool
	registryLock.Lock()
	if typ, ok = typeRegistry[t]; !ok {
		registryLock.Unlock()
		return ErrorType, nil, fmt.Errorf("Type %s not registered.", t.String())
	}
	registryLock.Unlock()
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
func (am *NetworkMessage) MarshalBinary() ([]byte, error) {
	return MarshalRegisteredType(am.Msg)
}

// UnmarshalBinary will decode the incoming bytes
// It uses protobuf for decoding (using the constructors in the NetworkMessage).
func (am *NetworkMessage) UnmarshalBinary(buf []byte) error {
	t, msg, err := UnmarshalRegisteredType(buf, am.Constructors)
	am.MsgType = t
	am.Msg = msg
	return err
}

// ConstructFrom takes a NetworkMessage and then constructs a
// NetworkMessage from it. Error if the type is unknown
func newNetworkMessage(obj ProtocolMessage) (*NetworkMessage, error) {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("Send takes a pointer to the message, not a copy...")
	}
	ty := TypeFromData(obj)
	if ty == ErrorType {
		return &NetworkMessage{}, fmt.Errorf("Packet to send is not known. Please register packet: %s\n",
			reflect.TypeOf(obj).String())
	}
	return &NetworkMessage{
		MsgType: ty,
		Msg:     obj}, nil
}
