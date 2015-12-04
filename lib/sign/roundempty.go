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
	dbg.Lvlf3("Making new RoundEmpty", node.Name())
	round := &RoundEmpty{}
	round.RoundStruct = NewRoundStruct(node, RoundEmptyType)
	// If you're sub-classing from another round-type, don't forget to remove
	// the above line, call the constructor of your parent round and add
	// round.Type = RoundEmptyType
	return round
}

func (round *RoundEmpty) Announcement(viewNbr, roundNbr int, in *SigningMessage, out []*SigningMessage) error {
	return nil
}

func (round *RoundEmpty) Commitment(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

func (round *RoundEmpty) Challenge(in *SigningMessage, out []*SigningMessage) error {
	return nil
}

func (round *RoundEmpty) Response(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

func (round *RoundEmpty) SignatureBroadcast(in *SigningMessage, out []*SigningMessage) error {
	return nil
}
