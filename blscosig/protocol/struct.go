package protocol

import (
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

// DefaultProtocolName can be used from other packages to refer to this protocol.
// If this name is used, then the suite used to verify signatures must be
// the default cothority.Suite.
const DefaultProtocolName = "bundleCoSiDefault"

func init() {
	network.RegisterMessages(&Rumor{}, &Shutdown{}, &Response{}, &Stop{})
}

// Rumor is a struct that can be sent in the gossip protocol
type Rumor struct {
	Params      Parameters
	ResponseMap map[uint32](*Response)
	Msg         []byte
	Data        []byte
}

// RumorMessage just contains a Rumor and the data necessary to identify and
// process the message in the onet framework.
type RumorMessage struct {
	*onet.TreeNode
	Rumor
}

// Shutdown is a struct that can be sent in the gossip protocol
// A valid shutdown message must contain a proof that the root has seen a valid
// final signature. This is to prevent faked shutdown messages that take down the
// gossip protocol. Thus the shutdown message contains the final signature,
// which in turn is signed by root.
type Shutdown struct {
	FinalCoSignature BlsSignature
	RootSig          []byte
}

// ShutdownMessage just contains a Shutdown and the data necessary to identify
// and process the message in the onet framework.
type ShutdownMessage struct {
	*onet.TreeNode
	Shutdown
}

// Response is the blscosi response message
type Response struct {
	Signature []byte
	Mask      []byte
}

// Refusal is the signed refusal response from a given node
type Refusal struct {
	Signature []byte
}

// StructRefusal contains the refusal and the treenode that sent it
type StructRefusal struct {
	*onet.TreeNode
	Refusal
}

// Stop is a message used to instruct a node to stop its protocol
type Stop struct{}

// StructStop is a wrapper around Stop for it to work with onet
type StructStop struct {
	*onet.TreeNode
	Stop
}
