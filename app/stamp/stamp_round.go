package main
import (
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/conode"
)

const RoundMeasureType = "measure"

type RoundMeasure struct {
	measure    *monitor.Measure
	firstRound int
	*conode.RoundStamperListener
}

// Pass firstround, as we will have some previous rounds to wait
// for everyone to be setup
func RegisterRoundMeasure(firstRound int) {
	sign.RegisterRoundFactory(RoundMeasureType,
		func(s *sign.Node) sign.Round {
			return NewRoundMeasure(s, firstRound)
		})
}

func NewRoundMeasure(node *sign.Node, firstRound int) *RoundMeasure {
	dbg.Lvlf3("Making new roundmeasure %+v", node)
	round := &RoundMeasure{}
	round.RoundStamperListener = conode.NewRoundStamperListener(node)
	round.Type = RoundMeasureType
	round.firstRound = firstRound
	return round
}

func (round *RoundMeasure) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	if round.IsRoot {
		round.measure = monitor.NewMeasure("round")
	}
	return round.RoundCosi.Announcement(viewNbr, roundNbr, in, out)
}

func (round *RoundMeasure)Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	err := round.RoundCosi.Response(in, out)
	if round.IsRoot {
		round.measure.Measure()
		dbg.Lvl1("Round", round.RoundNbr - round.firstRound + 1,
			"finished - took", round.measure.WallTime)
	}
	return err
}