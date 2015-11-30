package main

import (
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/dbg"
)

/*
ConodeStats implements a simple module that shows some statistics about the
actual connection.
 */

// The name type of this round implementation
const RoundStatsType = "conodestats"

type RoundStats struct {
	*conode.RoundStamperListener
	totalMessages int
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

func (round *RoundStats)Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	err := round.RoundStamperListener.Commitment(in, out)
	if round.IsRoot{
		round.totalMessages = out.Com.Messages
	}
	return err
}

func (round *RoundStats)SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	err := round.RoundStamperListener.SignatureBroadcast(in, out)
	if err == nil {
		round.totalMessages += in.SBm.Messages
		dbg.Lvlf1("This is round %d with %d messages - %d since start.",
			round.RoundNbr, round.totalMessages, in.SBm.Messages)
	}
	return err
}
