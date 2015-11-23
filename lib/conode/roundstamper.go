package conode
import (
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/dbg"
	"encoding/binary"
	"bytes"
	"time"
	"strconv"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/coconet"
)

/*
Implements a merkle-tree hasher for incoming messages that
are passed to roundcosi.
 */

const RoundStamperType = "stamper"

type RoundStamper struct {
	*sign.RoundStruct
	peer        *Peer
	Timestamp   int64

	Proof       []hashid.HashId // the inclusion-proof of the data
	MTRoot      hashid.HashId   // mt root for subtree, passed upwards
	StampLeaves []hashid.HashId
	StampRoot   hashid.HashId
	StampProofs []proof.Proof
	Queue       []ReplyMessage
}

type ReplyMessage struct {
	Val   []byte
	To    string
	ReqNo byte
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
	round.RoundStruct = sign.NewRoundStruct(round.peer.Node)
	in.Am.RoundType = RoundCosiStamperType
	if round.IsRoot {
		// We are root !
		// Adding timestamp
		ts := time.Now().UTC()
		var b bytes.Buffer
		round.Timestamp = ts.Unix()
		binary.Write(&b, binary.LittleEndian, ts.Unix())
		in.Am.Message = b.Bytes()
	} else {
		// otherwise decode it
		var t int64
		if err := binary.Read(bytes.NewBuffer(in.Am.Message), binary.LittleEndian, &t); err != nil {
			dbg.Lvl1("Unmashaling timestamp has failed")
		}
		round.Timestamp = t
	}

	round.SetRoundType(RoundCosiStamperType, out)
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
		round.StampRoot, round.StampProofs = proof.ProofTree(round.Suite.Hash, round.StampLeaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(round.Suite.Hash, round.StampRoot, round.StampLeaves, round.StampProofs) == true {
				dbg.Lvl4("Local Proofs of", round.Name, "successful for round " +
				strconv.Itoa(round.RoundNbr))
			} else {
				panic("Local Proofs" + round.Name + " unsuccessful for round " +
				strconv.Itoa(round.RoundNbr))
			}
		}
	}
	out.Com.MTRoot = round.StampRoot

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
	// Send back signature to clients
	for i, msg := range round.Queue {
		// proof to get from s.Root to big root
		combProof := make(proof.Proof, len(round.Proof))
		copy(combProof, round.Proof)

		// add my proof to get from a leaf message to my root s.Root
		combProof = append(combProof, round.StampProofs[i]...)

		// proof that I can get from a leaf message to the big root
		if proof.CheckProof(round.Suite.Hash, round.MTRoot,
			round.StampLeaves[i], combProof) {
			dbg.Lvl2("Proof is OK for msg", msg)
		} else {
			dbg.Lvl2("Inclusion-proof failed")
		}

		respMessg := &TimeStampMessage{
			Type:  StampSignatureType,
			ReqNo: SeqNo(msg.ReqNo),
			Srep: &StampSignature{
				SuiteStr:   round.Suite.String(),
				Timestamp:  round.Timestamp,
				MerkleRoot: round.MTRoot,
				Prf:        combProof,
				Response:   in.SBm.R0_hat,
				Challenge:  in.SBm.C,
				AggCommit:  in.SBm.V0_hat,
				AggPublic:  in.SBm.X0_hat,
			}}
		round.PutToClient(msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client", msg.To)
	}
	return nil
}


// Send message to client given by name
func (round *RoundStamper) PutToClient(name string, data coconet.BinaryMarshaler) {
	err := round.peer.Clients[name].PutData(data)
	if err == coconet.ErrClosed {
		round.peer.Clients[name].Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		dbg.Lvl1("%p error putting to client: %v", round, err)
	}
}
