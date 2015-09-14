package sign

import "github.com/dedis/crypto/abstract"
import "github.com/ineiti/cothorities/helpers/hashid"
import "github.com/ineiti/cothorities/helpers/proof"

const FIRST_ROUND int = 1 // start counting rounds at 1

type Round struct {
	c abstract.Secret // round lasting challenge
	r abstract.Secret // round lasting response

	Log       SNLog // round lasting log structure
	HashedLog []byte

	r_hat abstract.Secret // aggregate of responses
	X_hat abstract.Point  // aggregate of public keys

	Commits   []*SigningMessage
	Responses []*SigningMessage

	// own big merkle subtree
	MTRoot     hashid.HashId   // mt root for subtree, passed upwards
	Leaves     []hashid.HashId // leaves used to build the merkle subtre
	LeavesFrom []string        // child names for leaves

	// mtRoot before adding HashedLog
	LocalMTRoot hashid.HashId

	// merkle tree roots of children in strict order
	CMTRoots     []hashid.HashId
	CMTRootNames []string
	Proofs       map[string]proof.Proof

	// round-lasting public keys of children servers that did not
	// respond to latest commit or respond phase, in subtree
	ExceptionList []abstract.Point
	// combined point commits of children servers in subtree
	ChildV_hat map[string]abstract.Point
	// combined public keys of children servers in subtree
	ChildX_hat map[string]abstract.Point
	// for internal verification purposes
	exceptionV_hat abstract.Point

	BackLink hashid.HashId
	AccRound []byte

	Vote *Vote
	// VoteRequest  *VoteRequest  // Vote Request vote on in the round
	// CountedVotes *CountedVotes // CountedVotes contains a subtree's votes
}

func NewRound(suite abstract.Suite) *Round {
	round := &Round{}
	round.Commits = make([]*SigningMessage, 0)
	round.Responses = make([]*SigningMessage, 0)
	round.ExceptionList = make([]abstract.Point, 0)
	round.Log.Suite = suite
	return round
}

type RoundType int

const (
	EmptyRT RoundType = iota
	ViewChangeRT
	AddRT
	RemoveRT
	ShutdownRT
	NoOpRT
	SigningRT
)

func (rt RoundType) String() string {
	switch rt {
	case EmptyRT:
		return "empty"
	case SigningRT:
		return "signing"
	case ViewChangeRT:
		return "viewchange"
	case AddRT:
		return "add"
	case RemoveRT:
		return "remove"
	case ShutdownRT:
		return "shutdown"
	case NoOpRT:
		return "noop"
	default:
		return ""
	}
}
