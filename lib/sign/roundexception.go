package sign

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
)

/*
RoundException implements the collective signature protocol using
Schnorr signatures to collectively sign on a message. By default
the message is only the collection of all Commits, but another
round can add any message it wants in the Commitment-phase.
*/

// The name type of this round implementation
const RoundExceptionType = "exception"

// Can be used for debugging by telling which node should fail
// don't forget to reset it if used in a test otherwise it can interfere with other tests!
var ExceptionForceFailure string

type RoundException struct {
	*RoundCosi
}

// init adds RoundException to the list of available rounds
func init() {
	RegisterRoundFactory(RoundExceptionType,
		func(node *Node) Round {
			return NewRoundException(node)
		})
}

// NewRoundException creates a new RoundException based on RoundCosi
func NewRoundException(node *Node) *RoundException {
	dbg.Lvl3("Making new RoundException", node.Name())
	round := &RoundException{}
	round.RoundCosi = NewRoundCosi(node)
	round.Type = RoundExceptionType
	return round
}

// Commitment adds up all exception-lists from children and calls roundcosi
func (round *RoundException) Commitment(in []*CommitmentMessage, out *CommitmentMessage) error {
	err := round.RoundCosi.Commitment(in, out)
	if err != nil {
		return err
	}

	// prepare to handle exceptions
	cosi := round.Cosi
	cosi.RejectionPublicList = make([]abstract.Point, 0)
	for _, sm := range cosi.Commits {
		cosi.RejectionPublicList = append(cosi.RejectionPublicList, sm.RejectionPublicList...)
	}
	out.RejectionPublicList = round.Cosi.RejectionPublicList
	return nil
}

func (round *RoundException) Response(in []*ResponseMessage, out *ResponseMessage) error {
	if round.Name == ExceptionForceFailure {
		dbg.Lvl1("Forcing failure in response")
		round.RaiseException()
	}

	// initialize exception handling
	nullPoint := round.Cosi.Suite.Point().Null()

	children := round.Cosi.Children
	for _, sm := range in {
		from := sm.From
		// empty == no response from child
		if sm.Empty {
			dbg.Lvl4(round.Name, "Empty response from child", from)
			if children[from] != nil {
				round.Cosi.RejectionPublicList = append(round.Cosi.RejectionPublicList, children[from].Public)
				round.Cosi.RejectionCommitList = append(round.Cosi.RejectionCommitList, round.Cosi.ChildV_hat[from])

				// remove public keys and point commits from subtree of failed child
				round.Cosi.ExceptionX_hat.Add(round.Cosi.ExceptionX_hat, round.Cosi.ChildX_hat[from])
				round.Cosi.ExceptionV_hat.Add(round.Cosi.ExceptionV_hat, round.Cosi.ChildV_hat[from])
			}
		} else {
			// disregard response from children who did not commit
			_, ok := round.Cosi.ChildV_hat[from]
			if ok == true && round.Cosi.ChildV_hat[from].Equal(nullPoint) {
				dbg.Lvl4(round.Name, ": no response from", from)
				continue
			}

			dbg.Lvl4(round.Name, "accepts response from", from)
			round.Cosi.ExceptionV_hat.Add(round.Cosi.ExceptionV_hat, sm.ExceptionV_hat)
			round.Cosi.ExceptionX_hat.Add(round.Cosi.ExceptionX_hat, sm.ExceptionX_hat)
			round.Cosi.RejectionPublicList = append(round.Cosi.RejectionPublicList, sm.RejectionPublicList...)
			round.Cosi.RejectionCommitList = append(round.Cosi.RejectionCommitList, sm.RejectionCommitList...)
		}
	}

	round.Cosi.X_hat.Sub(round.Cosi.X_hat, round.Cosi.ExceptionX_hat)
	err := round.RoundCosi.Response(in, out)
	if err != nil {
		return err
	}

	out.RejectionPublicList = round.Cosi.RejectionPublicList
	out.RejectionCommitList = round.Cosi.RejectionCommitList

	out.ExceptionV_hat = round.Cosi.ExceptionV_hat
	out.ExceptionX_hat = round.Cosi.ExceptionX_hat
	return nil
}

func (round *RoundException) RaiseException() {
	round.Cosi.R_hat = round.Suite.Secret().Zero()
	round.Cosi.RejectionPublicList = append(round.Cosi.RejectionPublicList, round.Cosi.PubKey)
	// remove commitment of current node because it rejected to commit:
	round.Cosi.RejectionCommitList = append(round.Cosi.RejectionCommitList, round.Cosi.Log.V)
	round.Cosi.ExceptionX_hat.Add(round.Cosi.ExceptionX_hat, round.Cosi.PubKey)
	round.Cosi.ExceptionV_hat.Add(round.Cosi.ExceptionV_hat, round.Cosi.Log.V_hat)
}

func (round *RoundException) SignatureBroadcast(in *SignatureBroadcastMessage, out []*SignatureBroadcastMessage) error {
	// Root is creating the sig broadcast
	if round.IsRoot {
		in.RejectionPublicList = round.Cosi.RejectionPublicList
		in.RejectionCommitList = round.Cosi.RejectionCommitList
	}
	return round.RoundCosi.SignatureBroadcast(in, out)
}
