package main

import (
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
)

/*
ConodeStats implements a simple module that shows some statistics about the
actual connection.
*/

// The name type of this round implementation
const RoundStatsType = "conodestats"

type RoundStats struct {
	*conode.RoundStamperListener
}

func init() {
	sign.RegisterRoundFactory(RoundStatsType,
		func(node *sign.Node) sign.Round {
			return NewRoundStats(node)
		})
}

func NewRoundStats(node *sign.Node) *RoundStats {
	round := &RoundStats{}
	round.RoundStamperListener = conode.NewRoundStamperListener(node)
	return round
}

func (round *RoundStats) Commitment(in []*sign.CommitmentMessage, out *sign.CommitmentMessage) error {
	err := round.RoundStamperListener.Commitment(in, out)
	return err
}

func (round *RoundStats) SignatureBroadcast(in *sign.SignatureBroadcastMessage, out []*sign.SignatureBroadcastMessage) error {
	err := round.RoundStamperListener.SignatureBroadcast(in, out)
	if err == nil && round.IsRoot {
		dbg.Lvlf1("This is round %d with %d messages - %d since start.",
			round.RoundNbr, in.Messages, round.Node.Messages)
	}
	return err
}
