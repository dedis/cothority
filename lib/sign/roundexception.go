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
	dbg.Lvlf3("Making new RoundException", node.Name())
	round := &RoundException{}
	round.RoundCosi = NewRoundCosi(node)
	round.Type = RoundExceptionType
	return round
}

// Commitment adds up all exception-lists from children and calls roundcosi
func (round *RoundException) Commitment(in []*SigningMessage, out *SigningMessage) error {
	err := round.RoundCosi.Commitment(in, out)
	if err != nil {
		return err
	}

	// prepare to handle exceptions
	cosi := round.Cosi
	cosi.ExceptionList = make([]abstract.Point, 0)
	for _, sm := range cosi.Commits {
		cosi.ExceptionList = append(cosi.ExceptionList, sm.Com.ExceptionList...)
	}
	out.Com.ExceptionList = round.Cosi.ExceptionList
	return nil
}

func (round *RoundException) Response(in []*SigningMessage, out *SigningMessage) error {
	if round.Name == ExceptionForceFailure {
		dbg.Lvl1("Forcing failure in response")
		round.RaiseException()
	}

	// initialize exception handling
	nullPoint := round.Cosi.Suite.Point().Null()

	children := round.Cosi.Children
	for _, sm := range in {
		from := sm.From
		switch sm.Type {
		default:
			// default == no response from child
			dbg.Lvl4(round.Name, "Empty response from child", from, sm.Type)
			if children[from] != nil {
				// remove public keys and point commits from subtree of failed child
				round.Cosi.ExceptionX_hat.Add(round.Cosi.ExceptionX_hat, round.Cosi.ChildX_hat[from])
				round.Cosi.ExceptionV_hat.Add(round.Cosi.ExceptionV_hat, round.Cosi.ChildV_hat[from])
			}
			continue
		case Response:
			// disregard response from children who did not commit
			_, ok := round.Cosi.ChildV_hat[from]
			if ok == true && round.Cosi.ChildV_hat[from].Equal(nullPoint) {
				dbg.Lvl4(round.Name, ": no response from", from, sm.Type)
				continue
			}

			dbg.Lvl4(round.Name, "accepts response from", from, sm.Type)
			round.Cosi.ExceptionV_hat.Add(round.Cosi.ExceptionV_hat, sm.Rm.ExceptionV_hat)
			round.Cosi.ExceptionX_hat.Add(round.Cosi.ExceptionX_hat, sm.Rm.ExceptionX_hat)
		}
	}

	round.Cosi.X_hat.Sub(round.Cosi.X_hat, round.Cosi.ExceptionX_hat)
	// XXX Do we need to take care of V_hat here, too?
	err := round.RoundCosi.Response(in, out)
	if err != nil {
		return err
	}

	out.Rm.ExceptionV_hat = round.Cosi.ExceptionV_hat
	out.Rm.ExceptionX_hat = round.Cosi.ExceptionX_hat
	return nil
}

func (round *RoundException) RaiseException() {
	round.Cosi.R_hat = round.Suite.Secret().Zero()
	round.Cosi.ExceptionX_hat.Add(round.Cosi.ExceptionX_hat, round.Cosi.PubKey)
	//round.Cosi.ExceptionV_hat.Add(round.Cosi.ExceptionV_hat, round.Cosi.Log.V_hat)
}

func (round *RoundException) SignatureBroadcast(in *SigningMessage, out []*SigningMessage) error {
	// Root is creating the sig broadcast
	if round.IsRoot {
		in.SBm.ExceptionCommitList = append(in.SBm.ExceptionCommitList, round.Cosi.ExceptionV_hat)
		in.SBm.ExceptionPublicList = append(in.SBm.ExceptionPublicList, round.Cosi.ExceptionX_hat)

	}
	return round.RoundCosi.SignatureBroadcast(in, out)
}
