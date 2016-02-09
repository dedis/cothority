package bizcoin

type RoundType byte

const (
	ROUND_PREPARE RoundType = iota
	ROUND_COMMIT
)

type BizCoinAnnounce struct {
	cosi.Announcement
	TYPE RoundType
}

// announceChan is the type of the channel that will be used to catch
// announcement messges.
type announceChan struct {
	*sda.TreeNode
	BizCoinAnnounce
}

type BizCoinCommitment struct {
	TYPE RoundType
}

// commitChan is the type of the channel that will be used to catch commitment
// messages.
type commitChan struct {
	*sda.TreeNode
	BizCoinCommitment
}

// BizCoinChallengePrepare is the challenge used by BizCoin during the "prepare" phase. It contains the basic
// challenge plus the transactions from where the challenge has been generated.
type BizCoinChallengePrepare struct {
	cosi.Challenge
	blockchain.TrBlock
	TYPE RoundType
}

// BizCoinChallengeCommit  is the challenge used by BizCoin during the "commit"
// phase. It contains the basic challenge (out of the block we want to sign) +
// the signature of the "prepare" round.
type BizCoinChallengeCommit struct {
	TYPE RoundType
	cosi.Challenge
	// Signature is the basic signature Challenge / response
	Signature *cosi.Signature
	// Exception is the list of peers that did not want to sign. It's needed for
	// verifying the signature. It can not be spoofed otherwise the signature
	// would be wrong.
	Exceptions []Exception
}

// challengeChan is the type of the channel that will be used to dcatch the
// challenge messages.
type challengePrepareChan struct {
	*sda.TreeNode
	BizCoinChallengePrepare
}

type challengeCommitChan struct {
	*sda.TreeNode
	BizCoinChallengeCommit
}

// BizCoinResponse is the struct used by BizCoin during the response. It
// contains the response + a basic exception list.
type BizCoinResponse struct {
	cosi.Response
	Exceptions []Exception
	TYPE       RoundType
}

// responseChan is the type of the channel used to catch the response messages.
type responseChan struct {
	*sda.TreeNode
	BizCoinResponse
}

// Exception is what a node that does not want to sign should include when
// passing up a response
type Exception struct {
	Public     abstract.Point
	Commitment abstract.Point
}
