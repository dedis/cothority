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
const RoundExceptionType = "cosiexception"

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
	round := &RoundException{}
	round.RoundCosi = NewRoundCosi(node)
	round.Type = RoundExceptionType
	return round
}

/*
// Announcement only calls RoundCosi-Announcement, except if the round-name
// is equal to ExceptionForceFailure
func (round *RoundException) Announcement(viewNbr, roundNbr int, in *SigningMessage, out []*SigningMessage) error {
	dbg.Print(round.Name)
	if round.Name == ExceptionForceFailure {
		dbg.LLvl3("Forcing failure in announcement")
		return nil
	} else {
		return round.RoundCosi.Announcement(viewNbr, roundNbr, in, out)
	}
}
*/

// Commitment adds up all exception-lists from children and calls roundcosi
func (round *RoundException) Commitment(in []*SigningMessage, out *SigningMessage) error {
	/*
	if round.Name == ExceptionForceFailure {
		dbg.LLvl3("Forcing failure in commitment")
		return nil
	}
	*/

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

/*
func (round *RoundException) Challenge(in *SigningMessage, out []*SigningMessage) error {
	if round.Name == ExceptionForceFailure {
		dbg.LLvl3("Forcing failure in challenge")
		return nil
	} else {
		return round.RoundCosi.Challenge(in, out)
	}
}
*/

func (round *RoundException) Response(in []*SigningMessage, out *SigningMessage) error {
	/*
	if round.Name == ExceptionForceFailure {
		dbg.LLvl3("Forcing failure in response")
		return nil
	}
	*/

	// initialize exception handling
	exceptionV_hat := round.Cosi.Suite.Point().Null()
	exceptionX_hat := round.Cosi.Suite.Point().Null()
	round.Cosi.ExceptionList = make([]abstract.Point, 0)
	nullPoint := round.Cosi.Suite.Point().Null()

	children := round.Cosi.Children
	for _, sm := range in {
		from := sm.From
		switch sm.Type {
		default:
			// default == no response from child
			dbg.Lvl4(round.Name, "Empty response from child", from, sm.Type)
			if children[from] != nil {
				round.Cosi.ExceptionList = append(round.Cosi.ExceptionList, children[from].PubKey())

				// remove public keys and point commits from subtree of failed child
				exceptionX_hat.Add(exceptionX_hat, round.Cosi.ChildX_hat[from])
				exceptionV_hat.Add(exceptionV_hat, round.Cosi.ChildV_hat[from])
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
			exceptionV_hat.Add(exceptionV_hat, sm.Rm.ExceptionV_hat)
			exceptionX_hat.Add(exceptionX_hat, sm.Rm.ExceptionX_hat)
			round.Cosi.ExceptionList = append(round.Cosi.ExceptionList, sm.Rm.ExceptionList...)
		}
	}

	// remove exceptions from subtree that failed
	round.Cosi.X_hat.Sub(round.Cosi.X_hat, exceptionX_hat)
	round.Cosi.ExceptionV_hat = exceptionV_hat
	round.Cosi.ExceptionX_hat = exceptionX_hat

	err := round.RoundCosi.Response(in, out)
	if err != nil {
		return err
	}

	dbg.Lvl4(round.Cosi.Name, "got all responses")
	err = round.Cosi.VerifyResponses()
	if err != nil {
		dbg.Lvl3(round.Node.Name(), "Could not verify responses..")
		return err
	}

	out.Rm.ExceptionList = round.Cosi.ExceptionList
	out.Rm.ExceptionV_hat = round.Cosi.ExceptionV_hat
	out.Rm.ExceptionX_hat = round.Cosi.ExceptionX_hat
	return nil
}
