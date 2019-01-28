package protocol

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"crypto/cipher"
	"crypto/sha512"
	"hash"
	"time"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/group/edwards25519"
	"go.dedis.ch/kyber/v3/sign/cosi"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
)

// DefaultProtocolName can be used from other packages to refer to this protocol.
// If this name is used, then the suite used to verify signatures must be
// the default cothority.Suite.
const DefaultProtocolName = "ftCoSiProtoDefault"

// DefaultSubProtocolName the name of the default sub protocol, started by the
// main protocol.
const DefaultSubProtocolName = "ftSubCoSiProtoDefault"

type ftCosiSuite struct {
	cosi.Suite
	r cipher.Stream
}

func (m *ftCosiSuite) Hash() hash.Hash {
	return sha512.New()
}

func (m *ftCosiSuite) RandomStream() cipher.Stream {
	return m.r
}

// EdDSACompatibleCosiSuite is a custom suite made to be compatible with eddsa because
// cothority.Suite uses sha256 but EdDSA uses sha512.
var EdDSACompatibleCosiSuite = &ftCosiSuite{edwards25519.NewBlakeSHA256Ed25519(), random.New()}

// Announcement is the announcement message, the first message in the CoSi protocol
type Announcement struct {
	Msg       []byte
	Data      []byte
	Publics   []kyber.Point
	Timeout   time.Duration
	Threshold int
}

// StructAnnouncement just contains Announcement and the data necessary to identify and
// process the message in the onet framework.
type StructAnnouncement struct {
	*onet.TreeNode //sender
	Announcement
}

// Commitment is the ftcosi commitment message
type Commitment struct {
	CoSiCommitment kyber.Point
	Mask           []byte
	NRefusal       int
}

// StructCommitment just contains Commitment and the data necessary to identify and
// process the message in the onet framework.
type StructCommitment struct {
	*onet.TreeNode
	Commitment
}

// Challenge is the ftcosi challenge message
type Challenge struct {
	// CoSiChallenge is deprecated and should not be used anymore!
	CoSiChallenge kyber.Scalar
	// AggregateCommit should be used by all nodes.
	AggregateCommit kyber.Point
	// Mask represents the nodes that participated in the signature.
	Mask []byte
}

// StructChallenge just contains Challenge and the data necessary to identify and
// process the message in the onet framework.
type StructChallenge struct {
	*onet.TreeNode
	Challenge
}

// Response is the ftcosi response message
type Response struct {
	CoSiReponse kyber.Scalar
}

// StructResponse just contains Response and the data necessary to identify and
// process the message in the onet framework.
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
