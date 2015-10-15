package network

import (
	"bytes"
	"errors"
	"github.com/dedis/crypto/abstract"
)

// This file contains usual packets that are needed for different
// protocols.

// Type for MessageSigning
var MessageSigningType Type

// Type for BasicSignature
var BasicSignatureType Type

// Type for ListBasicSignature
var ListBasicSignatureType Type

// Init registers these few types on the network type registry
func init() {
	MessageSigningType = RegisterProtocolType(MessageSigning{})
	BasicSignatureType = RegisterProtocolType(BasicSignature{})
	ListBasicSignatureType = RegisterProtocolType(ListBasicSignature{})
}

// BasicSignatur is used to transmit our any kind of signature
// along with the public key ( used mostly for testing )
type BasicSignature struct {
	Pub   abstract.Point
	Chall abstract.Secret
	Resp  abstract.Secret
}

// ListBasicSignature is a packet representing a list of basic signature
// It is self-decodable by implementing Unmarshal binary interface
type ListBasicSignature struct {
	Length int
	Sigs   []BasicSignature
}

// UnmarshalBinary is  our custom decoding function
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

// MessageSigning is a simple packet to transmit a variable-length message
type MessageSigning struct {
	Length int
	Msg    []byte
}

// MarshalBinary encode MessageSigning it self. Shown mostly as an example
// as there is no need here to implement that since abstract.Encoding does
// it already
func (m *MessageSigning) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	err := Suite.Write(&b, m.Length, m.Msg)
	by := b.Bytes()
	return by, err
}

// UnmarshalBinary is needed to construct the slice containing the msg
// of the right length before decoding it from abstract.Encoding
func (m *MessageSigning) UnmarshalBinary(buf []byte) error {
	b := bytes.NewBuffer(buf)
	err := Suite.Read(b, &m.Length)
	if err != nil {
		return err
	}
	m.Msg = make([]byte, m.Length)
	err = Suite.Read(b, m.Msg)
	return err
}
