package conode
import (
	"github.com/dedis/cothority/lib/sign"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"encoding/binary"
	"bytes"
	"time"
)

/*
Implements the basic Collective Signature using Schnorr signatures.
 */

const RoundStamperType = "stamper"

type RoundStamper struct {
	*RoundStructure
	peer      *Peer
	Timestamp int64
}

func RegisterRoundStamper(p *Peer) {
	sign.RegisterRoundFactory(RoundStamperType,
		func(s *sign.Node) sign.Round {
			return NewRoundStamper(p)
		})
}

func NewRoundStamper(peer *Peer) *RoundStamper {
	round := &RoundStamper{peer: peer}
	return round
}

func (round *RoundStamper) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	dbg.Lvl3("New roundstamper announcement in round-nbr", roundNbr)
	round.RoundStructure = NewRoundStructure(round.peer.Node, viewNbr, roundNbr)
	in.Am.RoundType = RoundCosiStamperType
	if round.isRoot {
		// We are root !
		// Adding timestamp
		ts := time.Now().UTC()
		var b bytes.Buffer
		round.Timestamp = ts.Unix()
		binary.Write(&b, binary.LittleEndian, ts.Unix())
		in.Am.Message = b.Bytes()
		in.Am.RoundType = RoundCosiType
	} else {
		// otherwise decode it
		var t int64
		if err := binary.Read(bytes.NewBuffer(in.Am.Message), binary.LittleEndian, &t); err != nil {
			dbg.Lvl1("Unmashaling timestamp has failed")
		}
		round.Timestamp = t
	}

	return nil
}

func (round *RoundStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {

	return nil
}

func (round *RoundStamper) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func (round *RoundStamper) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (round *RoundStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}
