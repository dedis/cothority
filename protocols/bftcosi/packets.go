package bftcosi

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/cosi"
)

// RoundType is a type to know if we are in the "prepare" round or the "commit"
// round
type RoundType int32

const (
	// RoundPrepare is the first round (prepare)
	RoundPrepare RoundType = iota
	// RoundCommit is the final round (Commit)
	RoundCommit
)

// BFTSignature is what a bftcosi protocol outputs. It contains the signature,
// the message and some possible exceptions.
type BFTSignature struct {
	// cosi signature of the commit round.
	Sig []byte
	Msg []byte
	// List of peers that did not want to sign.
	Exceptions []Exception
}

// Verify returns whether the verification of the signature succeeds or not.
func (bs *BFTSignature) Verify(s abstract.Suite, agg abstract.Point, msg []byte) error {
	return cosi.VerifyCosiSignatureWithException(s, agg, msg, bs.Sig, bs.Exceptions)
}

// Announce is the struct used during the announcement phase (of both
// rounds)
type Announce struct {
	TYPE    RoundType
	Timeout uint64
}

// announceChan is the type of the channel that will be used to catch
// announcement messages.
type announceChan struct {
	*sda.TreeNode
	Announce
}

// Commitment is the commitment packets that is sent for both rounds
type Commitment struct {
	TYPE       RoundType
	Commitment abstract.Point
}

// commitChan is the type of the channel that will be used to catch commitment
// messages.
type commitChan struct {
	*sda.TreeNode
	Commitment
}

// ChallengePrepare is the challenge used by ByzCoin during the "prepare" phase.
// It contains the basic challenge plus the message from which the challenge has
// been generated.
type ChallengePrepare struct {
	Msg       []byte
	Data      []byte
	Challenge abstract.Scalar
}

// ChallengeCommit  is the challenge used by BftCoSi during the "commit"
// phase. It contains the basic challenge (out of the block we want to sign) +
// the signature of the "prepare" round. It also contains the exception list
// coming from the "prepare" phase. This exception list has been collected by
// the root during the response of the "prepare" phase and broadcast it through
// the challenge of the "commit". These are needed in order to verify the
// signature and to see how many peers did not sign. It's not spoofable because
// otherwise the signature verification will be wrong.
type ChallengeCommit struct {
	Challenge abstract.Scalar
	// Signature is the basic signature Challenge / response
	Signature []byte
	// Exception is the list of peers that did not want to sign. It's needed for
	// verifying the signature. It can not be spoofed otherwise the signature
	// would be wrong.
	Exceptions []Exception
}

// challengeChan is the type of the channel that will be used to catch the
// challenge messages.
type challengePrepareChan struct {
	*sda.TreeNode
	ChallengePrepare
}

// challengeCommitChan is the type of the channel used to catch the response messages.
type challengeCommitChan struct {
	*sda.TreeNode
	ChallengeCommit
}

// Response is the struct used by ByzCoin during the response. It
// contains the response + the basic exception list.
type Response struct {
	Response   abstract.Scalar
	Exceptions []Exception
	TYPE       RoundType
}

// responseChan is the type of the channel used to catch the response messages.
type responseChan struct {
	*sda.TreeNode
	Response
}

// Exception represents the exception mechanism used in BFTCosi to indicate a
// signer did not want to sign.
// The index is the index of the public key of the cosigner that do not want to
// sign.
// The commit is needed in order to be able to
// correctly verify the signature
type Exception struct {
	Index  int
	Commit abstract.Point
}
