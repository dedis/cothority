package sign

import (
	"bytes"
	"errors"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"sort"
)

const FIRST_ROUND int = 1 // start counting rounds at 1

type RoundMerkle struct {
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

func NewMerkle(suite abstract.Suite) *RoundMerkle {
	round := &RoundMerkle{}
	return round
}

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

/*
 * This is a module for the round-struct that does all the
 * calculation for a merkle-hash-tree.
 */

// Figure out which kids did not submit messages
// Add default messages to messgs, one per missing child
// as to make it easier to identify and add them to exception lists in one place
func (round *RoundMerkle) FillInWithDefaultMessages() []*SigningMessage {
	children := round.Children

	messgs := round.Responses
	allmessgs := make([]*SigningMessage, len(messgs))
	copy(allmessgs, messgs)

	for c := range children {
		found := false
		for _, m := range messgs {
			if m.From == c {
				found = true
				break
			}
		}

		if !found {
			allmessgs = append(allmessgs, &SigningMessage{View: round.View,
				Type: Default, From: c})
		}
	}

	return allmessgs
}

// Send children challenges
func (round *RoundMerkle) SendChildrenChallenges(chm *ChallengeMessage) error {
	for _, child := range round.Children {
		var messg coconet.BinaryMarshaler
		messg = &SigningMessage{View: round.View, Type: Challenge, Chm: chm}

		if err := child.PutData(messg); err != nil {
			return err
		}
	}

	return nil
}
