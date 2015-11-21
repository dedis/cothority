package conode
import (
	"github.com/dedis/cothority/lib/sign"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"encoding/binary"
	"bytes"
	"time"
	"strconv"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/hashid"
)

/*
Implements the basic Collective Signature using Schnorr signatures.
 */

const RoundStamperType = "stamper"

type RoundStamper struct {
	*RoundStructure
	peer        *Peer
	Timestamp   int64

	StampLeaves []hashid.HashId // can be removed after we verify protocol
	StampRoot   hashid.HashId
	StampProofs []proof.Proof
	Queue       []ReplyMessage
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
	// compute the local Merkle root

	// give up if nothing to process
	if len(round.Queue) == 0 {
		round.StampRoot = make([]byte, hashid.Size)
		round.StampProofs = make([]proof.Proof, 1)
	} else {
		// pull out to be Merkle Tree leaves
		round.StampLeaves = make([]hashid.HashId, 0)
		for _, msg := range round.Queue {
			round.StampLeaves = append(round.StampLeaves, hashid.HashId(msg.Val))
		}

		// create Merkle tree for this round's messages and check corectness
		round.StampRoot, round.StampProofs = proof.ProofTree(round.suite.Hash, round.StampLeaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(round.suite.Hash, round.StampRoot, round.StampLeaves, round.StampProofs) == true {
				dbg.Lvl4("Local Proofs of", round.name, "successful for round " +
				strconv.Itoa(round.roundNbr))
			} else {
				panic("Local Proofs" + round.name + " unsuccessful for round " +
				strconv.Itoa(round.roundNbr))
			}
		}
	}

	return nil
}

func (round *RoundStamper) QueueSet(Queue [][]MustReplyMessage) {
	// messages read will now be processed
	Queue[READING], Queue[PROCESSING] = Queue[PROCESSING], Queue[READING]
	Queue[READING] = Queue[READING][:0]
	round.Queue = make([]ReplyMessage, len(Queue[PROCESSING]))
	for i, q := range (Queue[PROCESSING]) {
		round.Queue[i] = ReplyMessage{
			Val: q.Tsm.Sreq.Val,
			To: q.To,
			ReqNo: byte(q.Tsm.ReqNo),
		}
	}
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
