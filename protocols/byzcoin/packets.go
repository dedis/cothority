package byzcoin

import (
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
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

// BlockSignature is what a byzcoin protocol outputs. It contains the signature,
// the block and some possible exceptions.
type BlockSignature struct {
	// cosi signature of the commit round.
	Sig *cosi.Signature
	// the block signed.
	Block *blockchain.TrBlock
	// List of peers that did not want to sign.
	Exceptions []cosi.Exception
}

// Announce is the struct used during the announcement phase (of both
// rounds)
type Announce struct {
	*cosi.Announcement
	TYPE    RoundType
	Timeout uint64
}

// announceChan is the type of the channel that will be used to catch
// announcement messges.
type announceChan struct {
	*sda.TreeNode
	Announce
}

// Commitment is the commitment packets that is sent for both rounds
type Commitment struct {
	TYPE RoundType
	*cosi.Commitment
}

// commitChan is the type of the channel that will be used to catch commitment
// messages.
type commitChan struct {
	*sda.TreeNode
	Commitment
}

// ChallengePrepare is the challenge used by ByzCoin during the "prepare" phase. It contains the basic
// challenge plus the transactions from where the challenge has been generated.
type ChallengePrepare struct {
	TYPE RoundType
	*cosi.Challenge
	*blockchain.TrBlock
}

// ChallengeCommit  is the challenge used by ByzCoin during the "commit"
// phase. It contains the basic challenge (out of the block we want to sign) +
// the signature of the "prepare" round. It also contains the exception list
// coming from the "prepare" phase. This exception list has been collected by
// the root during the response of the "prepare" phase and broadcast it through
// the challenge of the "commit". These are needed in order to verify the
// signature and to see how many peers did not sign. It's not spoofable because
// otherwise the signature verification will be wrong.
type ChallengeCommit struct {
	TYPE RoundType
	*cosi.Challenge
	// Signature is the basic signature Challenge / response
	Signature *cosi.Signature
	// Exception is the list of peers that did not want to sign. It's needed for
	// verifying the signature. It can not be spoofed otherwise the signature
	// would be wrong.
	Exceptions []cosi.Exception
}

// challengeChan is the type of the channel that will be used to dcatch the
// challenge messages.
type challengePrepareChan struct {
	*sda.TreeNode
	ChallengePrepare
}

type challengeCommitChan struct {
	*sda.TreeNode
	ChallengeCommit
}

// Response is the struct used by ByzCoin during the response. It
// contains the response + the basic exception list.
type Response struct {
	*cosi.Response
	Exceptions []cosi.Exception
	TYPE       RoundType
}

// responseChan is the type of the channel used to catch the response messages.
type responseChan struct {
	*sda.TreeNode
	Response
}
