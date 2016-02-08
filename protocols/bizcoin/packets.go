package bizcoin

// announceChan is the type of the channel that will be used to catch
// announcement messges.
type announceChan struct {
	*sda.TreeNode
	cosi.Announcement
}

// commitChan is the type of the channel that will be used to catch commitment
// messages.
type commitChan struct {
	*sda.TreeNode
	cosi.Commiment
}

// BizCoinChallenge is the challenge used by BizCoin. It contains the basic
// challenge plus the transactions from where the challenge has been generated.
type BizCoinChallenge struct {
	cosi.Challenge
	blockchain.TrBlock
}

// challengeChan is the type of the channel that will be used to dcatch the
// challenge messages.
type challengeChan struct {
	*sda.TreeNode
	BizCoinChallenge
}

// BizCoinResponse is the struct used by BizCoin during the response. It
// contains the response + a basic exception list.
type BizCoinResponse struct {
	cosi.Response
	Exceptions []Exception
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
