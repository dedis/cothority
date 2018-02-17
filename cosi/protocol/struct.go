package protocol

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/onet"
)

// DefaultProtocolName can be used from other packages to refer to this protocol.
const DefaultProtocolName = "CoSiProtoDefault"

// DefaultSubProtocolName is started by the main protocol.
const DefaultSubProtocolName = "SubCoSiProtoDefault"

// DefaultProtocolTimeout is the primary timeout for the CoSi protocol
const DefaultProtocolTimeout = time.Second * 10

// DefaultSubleaderTimeout is the timeout for subleader's responses
const DefaultSubleaderTimeout = DefaultProtocolTimeout / 10

// DefaultLeavesTimeout is the timeout for responses from the leaves
const DefaultLeavesTimeout = DefaultProtocolTimeout / 20

// Announcement is the announcement message, the first message in the CoSi protocol
type Announcement struct {
	Proposal         []byte
	Publics          []kyber.Point
	SubleaderTimeout time.Duration
	LeafTimeout      time.Duration
}

// StructAnnouncement just contains Announcement and the data necessary to identify and
// process the message in the sda framework.
type StructAnnouncement struct {
	*onet.TreeNode //sender
	Announcement
}

// Commitment is the cosi commitment message
type Commitment struct {
	CoSiCommitment kyber.Point
	Mask           []byte
}

// StructCommitment just contains Commitment and the data necessary to identify and
// process the message in the sda framework.
type StructCommitment struct {
	*onet.TreeNode
	Commitment
}

// Challenge is the cosi challenge message
type Challenge struct {
	CoSiChallenge kyber.Scalar
}

// StructChallenge just contains Challenge and the data necessary to identify and
// process the message in the sda framework.
type StructChallenge struct {
	*onet.TreeNode
	Challenge
}

// Response is the cosi response message
type Response struct {
	CoSiReponse kyber.Scalar
}

// StructResponse just contains Response and the data necessary to identify and
// process the message in the sda framework.
type StructResponse struct {
	*onet.TreeNode
	Response
}

// Stop is a message used to instruct a node to stop its protocol
type Stop struct{}

// StructStop is a wrapper around Stop for it to work with onet
type StructStop struct {
	*onet.TreeNode
	Stop
}
