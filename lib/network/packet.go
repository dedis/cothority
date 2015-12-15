package network

import "github.com/dedis/crypto/abstract"

// This file contains usual packets that are needed for different
// protocols.

const (
	// Type for MessageSigning
	MessageSigningType = iota

	// Type for BasicSignature
	BasicSignatureType

	// Type for ListBasicSignature
	ListBasicSignatureType
)

// Init registers these few types on the network type registry
func init() {
	RegisterProtocolType(MessageSigningType, MessageSigning{})
	RegisterProtocolType(BasicSignatureType, BasicSignature{})
	RegisterProtocolType(ListBasicSignatureType, ListBasicSignature{})
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

// MessageSigning is a simple packet to transmit a variable-length message
type MessageSigning struct {
	Length int
	Msg    []byte
}
