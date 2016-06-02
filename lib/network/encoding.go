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

// MessageTypeID is the ID used to uniquely identify different registered messages
type MessageTypeID uuid.UUID

// String returns the canonical string representation of the MessageTypeID
func (mId MessageTypeID) String() string {
	return uuid.UUID(mId).String()
}

// NamespaceURL is the basic namespace used for uuid
// XXX should move that to external of the library as not every
// cothority/packages should be expected to use that.
const NamespaceURL = "https://dedis.epfl.ch/"

// NamespaceProtocolMessageType is the namespace used for MessageTypeID
const NamespaceProtocolMessageType = NamespaceURL + "/protocolType/"

// RegisterMessageType registers a custom "struct" / "packet" and returns the
// corresponding MessageTypeID.
// Simply pass your non-initialized struct.
func RegisterMessageType(msg ProtocolMessage) MessageTypeID {
	msgType := TypeToMessageTypeID(msg)
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	t := val.Type()
	return RegisterMessageUUID(msgType, t)
}

// RegisterMessageUUID can be used if the uuid and the type is already known
// NOTE: be sure to only registers VALUE message and not POINTERS to message.
func RegisterMessageUUID(mt MessageTypeID, rt reflect.Type) MessageTypeID {
	if _, typeRegistered := registry.get(mt); typeRegistered {
		return mt
	}
	registry.put(mt, rt)

	return mt
}

// TypeFromData returns the MessageTypeID corresponding to the given structure.
// It returns 'ErrorType' if the type wasn't found or an error occurred.
func TypeFromData(msg ProtocolMessage) MessageTypeID {
	msgType := TypeToMessageTypeID(msg)
	_, ok := registry.get(msgType)
	if !ok {
		return ErrorType
	}
	return msgType
}

// TypeToMessageTypeID converts a ProtocolMessage to a MessageTypeID
func TypeToMessageTypeID(msg ProtocolMessage) MessageTypeID {
	val := reflect.ValueOf(msg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	url := NamespaceProtocolMessageType + val.Type().String()
	u := uuid.NewV5(uuid.NamespaceURL, url)
	dbg.Lvl5("Reflecting", reflect.TypeOf(msg), "to", u)
	return MessageTypeID(u)
}

// RTypeToMessageTypeID converts a reflect.Type to a MessageTypeID
func RTypeToMessageTypeID(msg reflect.Type) MessageTypeID {
	url := NamespaceProtocolMessageType + msg.String()
	return MessageTypeID(uuid.NewV5(uuid.NamespaceURL, url))
}

// DumpTypes is used for debugging - it prints out all known types
func DumpTypes() {
	for t, m := range registry.types {
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
func (am *Message) Error() error {
	return am.err
}

// SetError is workaround so we can set the error after creation of the
// application message
func (am *Message) SetError(err error) {
	am.err = err
}

type typeRegistry struct {
	types map[MessageTypeID]reflect.Type
	lock  sync.Mutex
}

func newTypeRegistry() *typeRegistry {
	return &typeRegistry{
		types: make(map[MessageTypeID]reflect.Type),
		lock:  sync.Mutex{},
	}
}

// get returns the reflect.Type corresponding to the registered MessageTypeID
// an a boolean indicating if the type is actually registered or not.
func (tr *typeRegistry) get(id MessageTypeID) (reflect.Type, bool) {
	tr.lock.Lock()
	defer tr.lock.Unlock()
	t, ok := tr.types[id]
	return t, ok
}

// put stores the given type in the typeRegistry.
func (tr *typeRegistry) put(id MessageTypeID, typ reflect.Type) {
	tr.lock.Lock()
	defer tr.lock.Unlock()
	tr.types[id] = typ
}

var registry = newTypeRegistry()

var globalOrder = binary.BigEndian

// ErrorType is reserved by the network library. When you receive a message of
// ErrorType, it is generally because an error happened, then you can call
// Error() on it.
var ErrorType = MessageTypeID(uuid.Nil)

// EmptyApplicationMessage is the default empty message that is returned in case
// something went wrong.
//
// FIXME currently there seems no way with go1.6 for this to compile without repeating
// the definition of ErrorType above as MessageTypeID(uuid.Nil).
// Somehow it still gets inlined (maybe through the indirection).
// should be fixed properly in go1.7:
// https://github.com/golang/go/commit/feb2a5d6103dad76b6374c5f346e33d55612cb2a
var EmptyApplicationMessage = Message{MsgType: MessageTypeID(uuid.Nil)}

// global mutex for MarshalRegisteredType
var marshalLock sync.Mutex

// MarshalRegisteredType will marshal a struct with its respective type into a
// slice of bytes. That slice of bytes can be then decoded in
// UnmarshalRegisteredType.
func MarshalRegisteredType(data ProtocolMessage) ([]byte, error) {
	marshalLock.Lock()
	defer marshalLock.Unlock()
	var msgType MessageTypeID
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
// decode a message from a buffer.
// The type must be registered to the network library in order to be decodable.
func UnmarshalRegisteredType(buf []byte, constructors protobuf.Constructors) (MessageTypeID, ProtocolMessage, error) {
	b := bytes.NewBuffer(buf)
	var tID MessageTypeID
	if err := binary.Read(b, globalOrder, &tID); err != nil {
		return ErrorType, nil, err
	}
	var typ reflect.Type
	var ok bool
	if typ, ok = registry.get(tID); !ok {
		return ErrorType, nil, fmt.Errorf("Type %s not registered.",
			typ.Name())
	}
	ptrVal := reflect.New(typ)
	ptr := ptrVal.Interface()
	var err error
	if err = protobuf.DecodeWithConstructors(b.Bytes(), ptr, constructors); err != nil {
		return tID, ptrVal.Elem().Interface(), err
	}
	return tID, ptrVal.Elem().Interface(), nil
}

// MarshalBinary the application message => to bytes
// Implements BinaryMarshaler interface so it will be used when sending with protobuf
func (am *Message) MarshalBinary() ([]byte, error) {
	return MarshalRegisteredType(am.Msg)
}

// UnmarshalBinary will decode the incoming bytes
// It uses protobuf for decoding (using the constructors in the NetworkMessage).
func (am *Message) UnmarshalBinary(buf []byte) error {
	t, msg, err := UnmarshalRegisteredType(buf, am.Constructors)
	am.MsgType = t
	am.Msg = msg
	return err
}

// ConstructFrom takes a ProtocolMessage and then constructs a
// Message from it. Error if the type is unknown
func NewNetworkMessage(obj ProtocolMessage) (*Message, error) {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("Send takes a pointer to the message, not a copy...")
	}
	ty := TypeFromData(obj)
	if ty == ErrorType {
		return &Message{}, fmt.Errorf("Packet to send is not known. Please register packet: %s\n",
			reflect.TypeOf(obj).String())
	}
	return &Message{
		MsgType: ty,
		Msg:     obj}, nil
}
