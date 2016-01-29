package sign

import "github.com/dedis/cothority/lib/dbg"

/*
RoundEmpty is a bare-bones round implementation to be copy-pasted. It
already implements RoundStruct for your convenience.
*/

// The name type of this round implementation
const RoundEmptyType = "empty"

type RoundEmpty struct {
	*RoundStruct
}

func init() {
	RegisterRoundFactory(RoundEmptyType,
		func(node *Node) Round {
			return NewRoundEmpty(node)
		})
}

func NewRoundEmpty(node *Node) *RoundEmpty {
	dbg.Lvl3("Making new RoundEmpty", node.Name())
	round := &RoundEmpty{}
	round.RoundStruct = NewRoundStruct(node, RoundEmptyType)
	// If you're sub-classing from another round-type, don't forget to remove
	// the above line, call the constructor of your parent round and add
	// round.Type = RoundEmptyType
	return round
}

func (round *RoundEmpty) Announcement(viewNbr, roundNbr int, in *AnnouncementMessage, out []*AnnouncementMessage) error {
	return nil
}

func (round *RoundEmpty) Commitment(in []*CommitmentMessage, out *CommitmentMessage) error {
	return nil
}

func (round *RoundEmpty) Challenge(in *ChallengeMessage, out []*ChallengeMessage) error {
	return nil
}

func (round *RoundEmpty) Response(in []*ResponseMessage, out *ResponseMessage) error {
	return nil
}

func (round *RoundEmpty) SignatureBroadcast(in *SignatureBroadcastMessage, out []*SignatureBroadcastMessage) error {
	return nil
}
