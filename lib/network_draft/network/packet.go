package network

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dedis/crypto/abstract"
	"io"
)

// This file contains usual packets that are needed for different
// protocols.

var MessageSigningType Type
var BasicSignatureType Type
var ListBasicSignatureType Type

func init() {
	MessageSigningType = RegisterProtocolType(MessageSigning{})
	BasicSignatureType = RegisterProtocolType(BasicSignature{})
	ListBasicSignatureType = RegisterProtocolType(ListBasicSignature{})
}

// Message used to transmit our generated signature
type BasicSignature struct {
	Pub   abstract.Point
	Chall abstract.Secret
	Resp  abstract.Secret
}

// Simply a packet representing a list of basic signature
type ListBasicSignature struct {
	Length int
	Sigs   []BasicSignature
}

// Now only need to implement this unmarshalbinary to
// get our custom decoding function
func (l *ListBasicSignature) UnmarshalBinary(buf []byte) error {
	b := bytes.NewBuffer(buf)
	// first decode size
	var length int
	err := Suite.Read(b, &length)
	if length < 0 {
		return errors.New("Received a length < 0 for ListBasicSignature msg")
	}
	l.Length = length
	l.Sigs = make([]BasicSignature, length)
	for i := range l.Sigs {
		err = Suite.Read(b, &l.Sigs[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// The message used to transmit our message to be signed
// TODO modify network so we can build packet that will be
// decoded with our own marshalbinary /unmarshal. We could
// then pass the size of the msg to be read before the msg
// itself.
type MessageSigning struct {
	Length int
	Msg    []byte
}

// Messagesigning must implement Marshaling interface
// so it can decode any variable length message
func (m *MessageSigning) String() string {
	return "MessageSigning " + fmt.Sprintf("%d", m.Length)
}
func (m *MessageSigning) MarshalSize() int {
	var b bytes.Buffer
	_ = Suite.Write(&b, m.Length)
	return len(m.Msg) + len(b.Bytes())
}
func (m *MessageSigning) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	err := Suite.Write(&b, m.Length, m.Msg)
	by := b.Bytes()
	return by, err
}

func (m *MessageSigning) MarshalTo(w io.Writer) (int, error) {
	err := Suite.Write(w, m.Length, m.Msg)
	return 0, err
}

// TODO really considerate wether UnmarshalFrom should return int or no..
// abstract encoding does not use it, nor does it use MarshalSIze...
func (m *MessageSigning) UnmarshalFrom(r io.Reader) (int, error) {
	err := Suite.Read(r, &m.Length)
	if err != nil {
		return 0, err
	}
	m.Msg = make([]byte, m.Length)
	err = Suite.Read(r, m.Msg)
	return 0, err
}

func (m *MessageSigning) UnmarshalBinary(buf []byte) error {
	b := bytes.NewBuffer(buf)
	_, err := m.UnmarshalFrom(b)
	return err
}
