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

type RoundException struct {
	*RoundCosi
}

func init() {
	RegisterRoundFactory(RoundExceptionType,
		func(node *Node) Round {
			return NewRoundException(node)
		})
}

func NewRoundException(node *Node) *RoundException {
	round := &RoundException{}
	round.RoundCosi = NewRoundCosi(node)
	return round
}

// AnnounceFunc will keep the timestamp generated for this round

func (round *RoundException) Commitment(in []*SigningMessage, out *SigningMessage) error {
	err := round.RoundCosi.Commitment(in, out)
	if err != nil{
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
				continue
			}

			dbg.Lvl4(round.Name, "accepts response from", from, sm.Type)
			round.Cosi.R_hat.Add(round.Cosi.R_hat, sm.Rm.R_hat)
			exceptionV_hat.Add(exceptionV_hat, sm.Rm.ExceptionV_hat)
			exceptionX_hat.Add(exceptionX_hat, sm.Rm.ExceptionX_hat)
			round.Cosi.ExceptionList = append(round.Cosi.ExceptionList, sm.Rm.ExceptionList...)
		}
	}

	// remove exceptions from subtree that failed
	round.Cosi.X_hat.Sub(round.Cosi.X_hat, exceptionX_hat)
	round.Cosi.ExceptionV_hat = exceptionV_hat
	round.Cosi.ExceptionX_hat = exceptionX_hat

	dbg.Lvl4(round.Cosi.Name, "got all responses")
	err := round.Cosi.VerifyResponses()
	if err != nil {
		dbg.Lvl3(round.Node.Name(), "Could not verify responses..")
		return err
	}

	err = round.RoundCosi.Response(in, out)
	if err != nil{
		return err
	}

	out.Rm.ExceptionList = round.Cosi.ExceptionList
	out.Rm.ExceptionV_hat = round.Cosi.ExceptionV_hat
	out.Rm.ExceptionX_hat = round.Cosi.ExceptionX_hat
	return nil
}
